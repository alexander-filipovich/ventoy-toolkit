#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${PROJECT_ROOT}/docker/compose.yaml"

DISK=""
SIZE_BYTES=""
OUTPUT="./artifacts/ventoy-dev.img"
MARGIN_MIB=128
DRY_RUN=0

fail() { echo "[create-dev-image][error] $*" >&2; exit 1; }
usage() {
  cat >&2 <<'EOF'
Usage: scripts/create-dev-image.sh [options]
  --disk /dev/diskN
  --size-bytes N
  --output PATH
  --margin-mib N
  --dry-run
  -h, --help
EOF
}

while (($#)); do
  case "$1" in
    --disk|--size-bytes|--output|--margin-mib)
      (($# >= 2)) || fail "$1 requires a value"
      case "$1" in
        --disk) DISK="$2" ;;
        --size-bytes) SIZE_BYTES="$2" ;;
        --output) OUTPUT="$2" ;;
        --margin-mib) MARGIN_MIB="$2" ;;
      esac
      shift 2
      ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage; fail "unknown argument: $1" ;;
  esac
done

[[ -n "${DISK}" || -n "${SIZE_BYTES}" ]] || fail "provide either --disk or --size-bytes"
[[ -z "${DISK}" || -z "${SIZE_BYTES}" ]] || fail "use only one of --disk or --size-bytes"
[[ "${MARGIN_MIB}" =~ ^[0-9]+$ ]] || fail "--margin-mib must be an unsigned integer"

command -v docker >/dev/null 2>&1 || fail "docker not found"
docker compose version >/dev/null 2>&1 || fail "docker compose is not available"
docker compose -f "${COMPOSE_FILE}" build ventoy >/dev/null

if [[ -n "${DISK}" ]]; then
  SIZE_BYTES="$(diskutil info -plist "${DISK}" 2>/dev/null | plutil -extract TotalSize raw - 2>/dev/null || true)"
  [[ "${SIZE_BYTES}" =~ ^[0-9]+$ ]] || fail "failed to read valid size from diskutil for ${DISK}"
fi
[[ "${SIZE_BYTES}" =~ ^[0-9]+$ ]] || fail "--size-bytes must be an unsigned integer"

MARGIN_BYTES=$((MARGIN_MIB * 1024 * 1024))
(( SIZE_BYTES > MARGIN_BYTES )) || fail "disk_bytes must be greater than margin (${MARGIN_MIB} MiB)"
TARGET_BYTES=$((((SIZE_BYTES - MARGIN_BYTES) / (1024 * 1024)) * (1024 * 1024)))
(( TARGET_BYTES > 0 )) || fail "target_bytes computed as zero after alignment"

OUTPUT_ABS="${OUTPUT}"
[[ "${OUTPUT_ABS}" == /* ]] || OUTPUT_ABS="${PROJECT_ROOT}/${OUTPUT_ABS}"
OUTPUT_REL="${OUTPUT_ABS#${PROJECT_ROOT}/}"
[[ "${OUTPUT_REL}" != "${OUTPUT_ABS}" ]] || fail "output path must be inside project root: ${PROJECT_ROOT}"

echo "[create-dev-image] disk_bytes=${SIZE_BYTES}" >&2
echo "[create-dev-image] target_bytes=${TARGET_BYTES}" >&2
echo "[create-dev-image] output=${OUTPUT_ABS}" >&2
echo "[create-dev-image] margin_mib=${MARGIN_MIB}" >&2

if ((DRY_RUN)); then
  echo "[create-dev-image] dry-run: truncate -s ${TARGET_BYTES} ${OUTPUT_ABS}" >&2
  echo "[create-dev-image] dry-run: docker run --rm --privileged --entrypoint bash -v /dev:/dev -v ${PROJECT_ROOT}:/workspace -w /workspace ventoy-wrapper:dev -lc '<losetup + ventoy -I -s + fdisk>'" >&2
  exit 0
fi

mkdir -p "$(dirname "${OUTPUT_ABS}")"
rm -f "${OUTPUT_ABS}"
truncate -s "${TARGET_BYTES}" "${OUTPUT_ABS}"

docker run --rm --privileged --entrypoint bash \
  -v /dev:/dev -v "${PROJECT_ROOT}:/workspace" -w /workspace ventoy-wrapper:dev \
  -lc "set -euo pipefail; LOOP_DEV=\$(losetup --find --show \"${OUTPUT_REL}\"); trap 'losetup -d \"\$LOOP_DEV\"' EXIT; printf 'y\ny\n' | ventoy -I -s \"\$LOOP_DEV\"; fdisk -l \"\$LOOP_DEV\" >/dev/null"

IMAGE_LOGICAL_BYTES="$(stat -f%z "${OUTPUT_ABS}")"
IMAGE_ALLOCATED_BYTES="$(( $(du -k "${OUTPUT_ABS}" | awk '{print $1}') * 1024 ))"
[[ "${IMAGE_LOGICAL_BYTES}" =~ ^[0-9]+$ ]] || fail "failed to read logical image size"
[[ "${IMAGE_ALLOCATED_BYTES}" =~ ^[0-9]+$ ]] || fail "failed to read allocated image size"
(( IMAGE_ALLOCATED_BYTES > 0 )) || fail "image allocated bytes is zero; Ventoy installation likely failed"

echo "[create-dev-image] image_logical_bytes=${IMAGE_LOGICAL_BYTES}" >&2
echo "[create-dev-image] image_allocated_bytes=${IMAGE_ALLOCATED_BYTES}" >&2
echo "[create-dev-image] status=success" >&2
