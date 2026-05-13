#!/usr/bin/env bash
set -euo pipefail

IMAGE=""
DISK=""
MODE="full"
BS="4m"
DRY_RUN=0
CONFIRM=""

log() { echo "[write-image] $*" >&2; }
fail() { echo "[write-image][error] $*" >&2; exit 1; }
human_bytes() {
  awk -v b="$1" 'BEGIN{
    if (b >= 1073741824) { printf "%.2f GiB", b/1073741824; exit }
    if (b >= 1048576) { printf "%.2f MiB", b/1048576; exit }
    if (b >= 1024) { printf "%.1f KiB", b/1024; exit }
    printf "%d B", b
  }'
}
human_speed() {
  awk -v bytes="$1" -v secs="$2" 'BEGIN{
    if (secs <= 0) { print "0 B/s"; exit }
    s = bytes / secs
    if (s >= 1048576) { printf "%.2f MiB/s", s/1048576; exit }
    if (s >= 1024) { printf "%.1f KiB/s", s/1024; exit }
    printf "%.0f B/s", s
  }'
}

usage() {
  cat >&2 <<'EOF'
Usage: scripts/write-image.sh --image PATH --disk /dev/diskN [options]

Options:
  --mode full            Write mode (only full is supported in this script version)
  --bs VALUE             dd block size (default: 4m)
  --confirm diskN        Non-interactive confirmation token (e.g. disk4)
  --dry-run              Print steps without writing
  -h, --help             Show help
EOF
}

while (($#)); do
  case "$1" in
    --image|--disk|--mode|--bs|--confirm)
      (($# >= 2)) || fail "$1 requires a value"
      case "$1" in
        --image) IMAGE="$2" ;;
        --disk) DISK="$2" ;;
        --mode) MODE="$2" ;;
        --bs) BS="$2" ;;
        --confirm) CONFIRM="$2" ;;
      esac
      shift 2
      ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage; fail "unknown argument: $1" ;;
  esac
done

[[ -n "${IMAGE}" ]] || fail "--image is required"
[[ -n "${DISK}" ]] || fail "--disk is required"
[[ "${MODE}" == "full" ]] || fail "unsupported mode '${MODE}' (supported: full)"

command -v diskutil >/dev/null 2>&1 || fail "diskutil not found"
command -v plutil >/dev/null 2>&1 || fail "plutil not found"
command -v dd >/dev/null 2>&1 || fail "dd not found"
command -v stat >/dev/null 2>&1 || fail "stat not found"

[[ -f "${IMAGE}" ]] || fail "image not found: ${IMAGE}"

DISK_ID="${DISK##*/}"
DISK_ID="${DISK_ID#r}"
[[ "${DISK_ID}" =~ ^disk[0-9]+$ ]] || fail "--disk must look like /dev/diskN or diskN"
DISK_NODE="/dev/${DISK_ID}"
RAW_DISK_NODE="/dev/r${DISK_ID}"

INFO_PLIST="$(diskutil info -plist "${DISK_NODE}" 2>/dev/null || true)"
[[ -n "${INFO_PLIST}" ]] || fail "failed to read disk info for ${DISK_NODE}"

WHOLE_DISK="$(printf '%s' "${INFO_PLIST}" | plutil -extract WholeDisk raw - 2>/dev/null || true)"
INTERNAL_DISK="$(printf '%s' "${INFO_PLIST}" | plutil -extract Internal raw - 2>/dev/null || true)"
EXTERNAL_DISK="$(printf '%s' "${INFO_PLIST}" | plutil -extract RemovableMediaOrExternalDevice raw - 2>/dev/null || true)"
DISK_BYTES="$(printf '%s' "${INFO_PLIST}" | plutil -extract TotalSize raw - 2>/dev/null || true)"

[[ "${WHOLE_DISK}" == "true" ]] || fail "${DISK_NODE} is not a whole disk"
[[ "${INTERNAL_DISK}" == "false" ]] || fail "refusing internal disk ${DISK_NODE}"
[[ "${EXTERNAL_DISK}" == "true" ]] || fail "disk ${DISK_NODE} is not external/removable"
[[ "${DISK_BYTES}" =~ ^[0-9]+$ ]] || fail "failed to read disk size"

IMAGE_BYTES="$(stat -f%z "${IMAGE}")"
[[ "${IMAGE_BYTES}" =~ ^[0-9]+$ ]] || fail "failed to read image size"
(( IMAGE_BYTES <= DISK_BYTES )) || fail "image is larger than disk (${IMAGE_BYTES} > ${DISK_BYTES})"

log "image=${IMAGE}"
log "image_bytes=${IMAGE_BYTES}"
log "disk=${DISK_NODE}"
log "raw_disk=${RAW_DISK_NODE}"
log "disk_bytes=${DISK_BYTES}"
log "mode=${MODE}"
log "bs=${BS}"

if (( DRY_RUN )); then
  log "dry-run: diskutil unmountDisk ${DISK_NODE}"
  log "dry-run: dd if=${IMAGE} of=${RAW_DISK_NODE} bs=${BS} conv=sync"
  log "dry-run: sync"
  log "status=dry-run"
  exit 0
fi

(( EUID == 0 )) || fail "run as root (use sudo) for real writes"

if [[ -z "${CONFIRM}" ]]; then
  echo "Type '${DISK_ID}' to confirm writing ${IMAGE} to ${RAW_DISK_NODE}:" >&2
  read -r CONFIRM
fi
[[ "${CONFIRM}" == "${DISK_ID}" ]] || fail "confirmation mismatch (expected '${DISK_ID}')"

diskutil unmountDisk "${DISK_NODE}" >/dev/null

log "progress_backend=dd-siginfo"
dd if="${IMAGE}" of="${RAW_DISK_NODE}" bs="${BS}" conv=sync \
  2> >(
    while IFS= read -r line; do
      if [[ "${line}" =~ ^([0-9]+)\ bytes\ transferred\ in\ ([0-9.]+)\ secs ]]; then
        bytes="${BASH_REMATCH[1]}"
        secs="${BASH_REMATCH[2]}"
        pct=$(( bytes * 100 / IMAGE_BYTES ))
        (( pct > 100 )) && pct=100
        bytes_h="$(human_bytes "${bytes}")"
        total_h="$(human_bytes "${IMAGE_BYTES}")"
        speed_h="$(human_speed "${bytes}" "${secs}")"
        printf '\r[write-image] progress=%3d%% transferred=%s/%s speed=%s' "${pct}" "${bytes_h}" "${total_h}" "${speed_h}" >&2
      fi
    done
    printf '\n' >&2
  ) &
DD_PID=$!
while kill -0 "${DD_PID}" 2>/dev/null; do
  sleep 2
  kill -INFO "${DD_PID}" 2>/dev/null || true
done
wait "${DD_PID}"

sync

log "status=success"
