#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${PROJECT_ROOT}/docker/compose.yaml"

source "${SCRIPT_DIR}/common.sh"

SIZE_BYTES=""
OUTPUT="./artifacts/ventoy-dev.img"
DEFAULT_TARGET_MIB=64
DRY_RUN=0
METADATA_PATH=""

fail() { echo "[create-dev-image][error] $*" >&2; exit 1; }
cleanup() { rm -f "${PARTITION_JSON_ABS:-}"; }
usage() {
  cat >&2 <<'EOF'
Usage: scripts/create-dev-image.sh [options]
  --size-bytes N          Exact image size in bytes
  --output PATH
  --dry-run
  -h, --help
EOF
}

while (($#)); do
  case "$1" in
    --size-bytes|--output)
      (($# >= 2)) || fail "$1 requires a value"
      case "$1" in
        --size-bytes) SIZE_BYTES="$2" ;;
        --output) OUTPUT="$2" ;;
      esac
      shift 2
      ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage; fail "unknown argument: $1" ;;
  esac
done

if [[ -z "${SIZE_BYTES}" ]]; then
  SIZE_BYTES=$((DEFAULT_TARGET_MIB * 1024 * 1024))
fi
[[ "${SIZE_BYTES}" =~ ^[0-9]+$ ]] || fail "--size-bytes must be an unsigned integer"
TARGET_BYTES="${SIZE_BYTES}"
(( TARGET_BYTES > 0 )) || fail "target_bytes computed as zero after alignment"

OUTPUT_ABS="${OUTPUT}"
[[ "${OUTPUT_ABS}" == /* ]] || OUTPUT_ABS="${PROJECT_ROOT}/${OUTPUT_ABS}"
OUTPUT_REL="${OUTPUT_ABS#${PROJECT_ROOT}/}"
[[ "${OUTPUT_REL}" != "${OUTPUT_ABS}" ]] || fail "output path must be inside project root: ${PROJECT_ROOT}"
METADATA_PATH="${OUTPUT_ABS}.write-map.json"
PARTITION_JSON_ABS="${OUTPUT_ABS}.partition.json.tmp"
PARTITION_JSON_REL="${PARTITION_JSON_ABS#${PROJECT_ROOT}/}"
trap cleanup EXIT

echo "[create-dev-image] target_bytes=${TARGET_BYTES}" >&2
echo "[create-dev-image] output=${OUTPUT_ABS}" >&2

if ((DRY_RUN)); then
  echo "[create-dev-image] dry-run: truncate -s ${TARGET_BYTES} ${OUTPUT_ABS}" >&2
  echo "[create-dev-image] dry-run: docker run --rm --privileged --entrypoint bash -v /dev:/dev -v ${PROJECT_ROOT}:/workspace -w /workspace ventoy-wrapper:dev -lc '<losetup + ventoy -I -s + sfdisk -J>'" >&2
  echo "[create-dev-image] dry-run: ventoyctl map-image --image ${OUTPUT_ABS} --partition-json ${PARTITION_JSON_ABS}" >&2
  echo "[create-dev-image] dry-run: write metadata to ${METADATA_PATH}" >&2
  exit 0
fi

command -v docker >/dev/null 2>&1 || fail "docker not found"
docker compose version >/dev/null 2>&1 || fail "docker compose is not available"
docker compose -f "${COMPOSE_FILE}" build ventoy >/dev/null

mkdir -p "$(dirname "${OUTPUT_ABS}")"
rm -f "${OUTPUT_ABS}"
rm -f "${PARTITION_JSON_ABS}"
truncate -s "${TARGET_BYTES}" "${OUTPUT_ABS}"

docker run --rm --privileged --entrypoint bash \
  -v /dev:/dev -v "${PROJECT_ROOT}:/workspace" -w /workspace ventoy-wrapper:dev \
  -lc "set -euo pipefail; LOOP_DEV=\$(losetup --find --show \"${OUTPUT_REL}\"); trap 'losetup -d \"\$LOOP_DEV\"' EXIT; printf 'y\ny\n' | ventoy -I -s \"\$LOOP_DEV\"; sfdisk -J \"\$LOOP_DEV\" > \"${PARTITION_JSON_REL}\""

IMAGE_LOGICAL_BYTES="$(stat -f%z "${OUTPUT_ABS}")"
IMAGE_ALLOCATED_BYTES="$(( $(du -k "${OUTPUT_ABS}" | awk '{print $1}') * 1024 ))"
[[ "${IMAGE_LOGICAL_BYTES}" =~ ^[0-9]+$ ]] || fail "failed to read logical image size"
[[ "${IMAGE_ALLOCATED_BYTES}" =~ ^[0-9]+$ ]] || fail "failed to read allocated image size"
(( IMAGE_ALLOCATED_BYTES > 0 )) || fail "image allocated bytes is zero; Ventoy installation likely failed"

run_ventoyctl map-image \
  --image "${OUTPUT_ABS}" \
  --image-path "${OUTPUT_REL}" \
  --partition-json "${PARTITION_JSON_ABS}" \
  > "${METADATA_PATH}"

echo "[create-dev-image] image_logical_bytes=${IMAGE_LOGICAL_BYTES}" >&2
echo "[create-dev-image] image_allocated_bytes=${IMAGE_ALLOCATED_BYTES}" >&2
echo "[create-dev-image] write_map=${METADATA_PATH}" >&2
echo "[create-dev-image] status=success" >&2
