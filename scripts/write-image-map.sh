#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
source "${SCRIPT_DIR}/go-target.sh"

MAP_PATH="./artifacts/ventoy-dev.img.write-map.json"
DISK=""
TARGET_BYTES=""
CONFIRM=""
DRY_RUN=0
TMP_DIR=""

log() { echo "[write-image-map] $*" >&2; }
fail() { echo "[write-image-map][error] $*" >&2; exit 1; }
cleanup() { [[ -n "${TMP_DIR}" ]] && rm -rf "${TMP_DIR}"; }
trap cleanup EXIT

human_bytes() {
  awk -v b="$1" 'BEGIN{
    kib=2^10; mib=2^20; gib=2^30; tib=2^40
    if (b >= tib) { printf "%.2f TiB", b/tib; exit }
    if (b >= gib) { printf "%.2f GiB", b/gib; exit }
    if (b >= mib) { printf "%.2f MiB", b/mib; exit }
    if (b >= kib) { printf "%.1f KiB", b/kib; exit }
    printf "%d B", b
  }'
}

best_block_size() {
  local src_off="$1"
  local dst_off="$2"
  local len="$3"
  local candidate=""
  for candidate in $((4 * 1024 * 1024)) $((1024 * 1024)) $((64 * 1024)) "${SECTOR_SIZE}"; do
    if (( src_off % candidate == 0 && dst_off % candidate == 0 && len % candidate == 0 )); then
      printf '%s\n' "${candidate}"
      return
    fi
  done
  printf '%s\n' "${SECTOR_SIZE}"
}

copy_range() {
  local id="$1"
  local src_file="$2"
  local src_off="$3"
  local dst_off="$4"
  local len="$5"
  local bs="${SECTOR_SIZE}"
  local src_blocks=""
  local dst_blocks=""
  local count_blocks=""
  local progress_file="${TMP_DIR}/dd-${id}.log"
  local pid=""
  local current=0
  local total=0
  local elapsed=1
  local speed=0
  local pct=0

  bs="$(best_block_size "${src_off}" "${dst_off}" "${len}")"
  src_blocks=$((src_off / bs))
  dst_blocks=$((dst_off / bs))
  count_blocks=$((len / bs))

  : > "${progress_file}"
  dd if="${src_file}" of="${RAW_DISK_NODE}" bs="${bs}" skip="${src_blocks}" seek="${dst_blocks}" count="${count_blocks}" conv=notrunc 2>"${progress_file}" &
  pid="$!"

  while kill -0 "${pid}" 2>/dev/null; do
    sleep 1
    kill -INFO "${pid}" 2>/dev/null || true
    current="$(awk '/bytes transferred/ {bytes=$1} END {print bytes+0}' "${progress_file}")"
    total=$((written + current))
    elapsed=$(( $(date +%s) - start_ts ))
    (( elapsed <= 0 )) && elapsed=1
    speed=$(( total / elapsed ))
    pct=$(( total * 100 / WRITE_BYTES ))
    (( pct > 100 )) && pct=100
    printf '\r[write-image-map] progress=%3d%% written=%s/%s speed=%s/s range=%s' \
      "${pct}" "$(human_bytes "${total}")" "$(human_bytes "${WRITE_BYTES}")" "$(human_bytes "${speed}")" "${id}" >&2
  done

  if ! wait "${pid}"; then
    printf '\n' >&2
    cat "${progress_file}" >&2
    fail "dd failed while writing ${id}"
  fi
  current="$(awk '/bytes transferred/ {bytes=$1} END {print bytes+0}' "${progress_file}")"
  if (( current == 0 )); then
    current="${len}"
  fi
  written=$((written + current))
  elapsed=$(( $(date +%s) - start_ts ))
  (( elapsed <= 0 )) && elapsed=1
  speed=$(( written / elapsed ))
  pct=$(( written * 100 / WRITE_BYTES ))
  (( pct > 100 )) && pct=100
  printf '\r[write-image-map] progress=%3d%% written=%s/%s speed=%s/s range=%s\n' \
    "${pct}" "$(human_bytes "${written}")" "$(human_bytes "${WRITE_BYTES}")" "$(human_bytes "${speed}")" "${id}" >&2
}

usage() {
  cat >&2 <<'USAGE'
Usage: scripts/write-image-map.sh [--map PATH] [--disk /dev/diskN | --target-bytes N] [options]

Options:
  --target-bytes N  Build dry-run plan without touching a disk
  --confirm diskN   Non-interactive confirmation token (e.g. disk4)
  --dry-run         Print plan without writing
  -h, --help        Show help
USAGE
}

while (($#)); do
  case "$1" in
    --map|--disk|--target-bytes|--confirm)
      (($# >= 2)) || fail "$1 requires a value"
      case "$1" in
        --map) MAP_PATH="$2" ;;
        --disk) DISK="$2" ;;
        --target-bytes) TARGET_BYTES="$2" ;;
        --confirm) CONFIRM="$2" ;;
      esac
      shift 2
      ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage; fail "unknown argument: $1" ;;
  esac
done

command -v dd >/dev/null 2>&1 || fail "dd not found"
command -v stat >/dev/null 2>&1 || fail "stat not found"

MAP_ABS="${MAP_PATH}"
[[ "${MAP_ABS}" == /* ]] || MAP_ABS="${PROJECT_ROOT}/${MAP_ABS}"
[[ -f "${MAP_ABS}" ]] || fail "map file not found: ${MAP_ABS}"

if [[ -n "${DISK}" ]]; then
  DISK_ID="${DISK##*/}"
  DISK_ID="${DISK_ID#r}"
  [[ "${DISK_ID}" =~ ^disk[0-9]+$ ]] || fail "--disk must look like /dev/diskN or diskN"
  DISK_NODE="/dev/${DISK_ID}"
  RAW_DISK_NODE="/dev/r${DISK_ID}"
else
  DISK_ID=""
  DISK_NODE=""
  RAW_DISK_NODE=""
fi

[[ -z "${TARGET_BYTES}" || "${TARGET_BYTES}" =~ ^[0-9]+$ ]] || fail "--target-bytes must be an unsigned integer"
[[ -z "${DISK}" || -z "${TARGET_BYTES}" ]] || fail "use only one of --disk or --target-bytes"
[[ -n "${DISK}" || -n "${TARGET_BYTES}" ]] || fail "--disk is required (or --target-bytes with --dry-run)"
[[ -z "${TARGET_BYTES}" || "${DRY_RUN}" == "1" ]] || fail "--target-bytes is only for --dry-run"

if (( DRY_RUN == 0 )); then
  (( EUID == 0 )) || fail "run as root (use sudo) for real writes"
fi

if [[ -n "${DISK}" ]]; then
  command -v diskutil >/dev/null 2>&1 || fail "diskutil not found"
  command -v plutil >/dev/null 2>&1 || fail "plutil not found"

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
else
  DISK_BYTES="${TARGET_BYTES}"
fi

TMP_DIR="$(mktemp -d)"
META_ENV="${TMP_DIR}/meta.env"
RANGES_TSV="${TMP_DIR}/ranges.tsv"

run_go_target write-plan "${GO_TOOL_MODE:-binary}" \
  --map "${MAP_ABS}" \
  --target-bytes "${DISK_BYTES}" \
  --env "${META_ENV}" \
  --ranges "${RANGES_TSV}"

# shellcheck disable=SC1090
source "${META_ENV}"

IMAGE_ABS="${IMAGE_PATH}"
[[ "${IMAGE_ABS}" == /* ]] || IMAGE_ABS="${PROJECT_ROOT}/${IMAGE_ABS}"
[[ -f "${IMAGE_ABS}" ]] || fail "image referenced in map not found: ${IMAGE_ABS}"

IMAGE_BYTES="$(stat -f%z "${IMAGE_ABS}")"
[[ "${IMAGE_BYTES}" =~ ^[0-9]+$ ]] || fail "failed to read image size"
(( IMAGE_BYTES == IMAGE_LOGICAL_BYTES )) || fail "image size mismatch (map=${IMAGE_LOGICAL_BYTES}, file=${IMAGE_BYTES})"

log "map=${MAP_ABS}"
log "image=${IMAGE_ABS}"
log "disk=${DISK_NODE:-<dry-run target>}"
log "raw_disk=${RAW_DISK_NODE:-/dev/rdiskN}"
log "image_logical_size=$(human_bytes "${IMAGE_LOGICAL_BYTES}") (${IMAGE_LOGICAL_BYTES} B)"
log "target_size=$(human_bytes "${DISK_BYTES}") (${DISK_BYTES} B)"
log "sector_size=${SECTOR_SIZE}"
log "p1_start_sector=${P1_START_SECTOR}"
log "p1_old_size=$(human_bytes "$((P1_OLD_SIZE_SECTORS * SECTOR_SIZE))") (${P1_OLD_SIZE_SECTORS} sectors)"
log "p1_new_size=$(human_bytes "$((P1_NEW_SIZE_SECTORS * SECTOR_SIZE))") (${P1_NEW_SIZE_SECTORS} sectors)"
log "p2_old_start_sector=${P2_OLD_START_SECTOR}"
log "p2_new_start_sector=${P2_NEW_START_SECTOR}"
log "p2_size=$(human_bytes "$((P2_SIZE_SECTORS * SECTOR_SIZE))") (${P2_SIZE_SECTORS} sectors)"
log "write_bytes_total=$(human_bytes "${WRITE_BYTES}") (${WRITE_BYTES} B)"

print_ranges() {
  local prefix="$1"
  local out_disk="${RAW_DISK_NODE:-/dev/rdiskN}"
  local bs=""
  while IFS=$'\t' read -r id src_file src_off dst_off len; do
    [[ -n "${id}" ]] || continue
    [[ "${src_file}" == "__IMAGE__" ]] && src_file="${IMAGE_ABS}"
    bs="$(best_block_size "${src_off}" "${dst_off}" "${len}")"
    log "${prefix}: ${id}: dd if=${src_file} of=${out_disk} bs=${bs} skip=$((src_off / bs)) seek=$((dst_off / bs)) count=$((len / bs)) conv=notrunc"
  done < "${RANGES_TSV}"
}

if (( DRY_RUN )); then
  [[ -n "${DISK_NODE}" ]] && log "dry-run: diskutil unmountDisk force ${DISK_NODE}"
  print_ranges "dry-run"
  log "dry-run: diskutil eraseVolume ExFAT Ventoy ${DISK_NODE:-/dev/diskN}s1"
  log "dry-run: sync"
  log "status=dry-run"
  exit 0
fi

if [[ -z "${CONFIRM}" ]]; then
  echo "Type '${DISK_ID}' to confirm writing Ventoy layout to ${RAW_DISK_NODE} and formatting ${DISK_NODE}s1:" >&2
  read -r CONFIRM
fi
[[ "${CONFIRM}" == "${DISK_ID}" ]] || fail "confirmation mismatch (expected '${DISK_ID}')"

diskutil unmountDisk force "${DISK_NODE}" >/dev/null || true

start_ts="$(date +%s)"
written=0
log "copy_start"
while IFS=$'\t' read -r id src_file src_off dst_off len; do
  [[ -n "${id}" ]] || continue
  [[ "${src_file}" == "__IMAGE__" ]] && src_file="${IMAGE_ABS}"
  (( src_off % SECTOR_SIZE == 0 )) || fail "${id} source offset is not sector-aligned"
  (( dst_off % SECTOR_SIZE == 0 )) || fail "${id} target offset is not sector-aligned"
  (( len % SECTOR_SIZE == 0 )) || fail "${id} length is not sector-aligned"

  diskutil unmountDisk force "${DISK_NODE}" >/dev/null || true
  log "${id}: src=${src_file} src_offset=${src_off} dst_offset=${dst_off} length=${len}"
  copy_range "${id}" "${src_file}" "${src_off}" "${dst_off}" "${len}"
done < "${RANGES_TSV}"

sync
log "format_p1_start=${DISK_NODE}s1"
diskutil list "${DISK_NODE}" >/dev/null || true
P1_INFO_PLIST="$(diskutil info -plist "${DISK_NODE}s1" 2>/dev/null || true)"
[[ -n "${P1_INFO_PLIST}" ]] || fail "macOS did not expose ${DISK_NODE}s1 after writing partition table"
P1_OFFSET="$(printf '%s' "${P1_INFO_PLIST}" | plutil -extract PartitionMapPartitionOffset raw - 2>/dev/null || true)"
if [[ -n "${P1_OFFSET}" && "${P1_OFFSET}" =~ ^[0-9]+$ ]]; then
  (( P1_OFFSET == P1_START_SECTOR * SECTOR_SIZE )) || fail "${DISK_NODE}s1 offset mismatch after write"
fi
diskutil unmountDisk force "${DISK_NODE}" >/dev/null || true
diskutil eraseVolume ExFAT Ventoy "${DISK_NODE}s1" >/dev/null
sync
log "status=success"
