#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="${PROJECT_ROOT}/scripts"
DEFAULT_IMAGE="./artifacts/ventoy-dev.img"

source "${SCRIPTS_DIR}/common.sh"

CMD="" DISK="" IMAGE="" OUTPUT="" SIZE="" SIZE_BYTES="" CONFIRM="" DRY_RUN=0 FORCE=0 NO_BUILD="${NO_BUILD:-0}"

fail() { echo "[flow][error] $*" >&2; exit 1; }

usage() {
  cat >&2 <<'EOF'
Usage:
  ./ventoy-flow.sh [--disk diskN] [--confirm diskN] [--no-build] [--dry-run]
  ./ventoy-flow.sh list
  ./ventoy-flow.sh build
  ./ventoy-flow.sh create [--size 128m|--size-bytes N] [--output PATH] [--force] [--no-build] [--dry-run]
  ./ventoy-flow.sh write --disk diskN [--image PATH] [--confirm diskN] [--no-build] [--dry-run]
  ./ventoy-flow.sh all [--disk diskN] [--output PATH] [--confirm diskN] [--force] [--no-build] [--dry-run]
EOF
}

parse_size_to_bytes() {
  local value="$1"
  local lower num suffix mult
  lower="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')"
  [[ "$lower" =~ ^([0-9]+)([gmkb])$ ]] || fail "invalid --size '${value}'"
  num="${BASH_REMATCH[1]}"
  suffix="${BASH_REMATCH[2]}"
  case "$suffix" in
    g) mult=$((1024 * 1024 * 1024)) ;;
    m) mult=$((1024 * 1024)) ;;
    k) mult=1024 ;;
    b) mult=1 ;;
    *) fail "invalid --size suffix in '${value}'" ;;
  esac
  printf '%s' "$((num * mult))"
}

select_disk() {
  local args=(select-disk)
  if (( DRY_RUN )); then
    args+=(--dry-run)
  fi
  DISK="$(run_ventoyctl "${args[@]}")"
  [[ -n "$DISK" ]] || fail "selector returned empty disk"
}

run_create() {
  local args=()

  [[ "$CMD" != "create" || -z "$DISK" ]] || fail "--disk is used by write/all, not create"
  [[ -z "$SIZE" || -z "$SIZE_BYTES" ]] || fail "use only one size flag"

  if [[ -n "$SIZE" ]]; then
    args+=(--size-bytes "$(parse_size_to_bytes "$SIZE")")
  elif [[ -n "$SIZE_BYTES" ]]; then
    [[ "$SIZE_BYTES" =~ ^[0-9]+$ ]] || fail "--size-bytes must be an unsigned integer"
    args+=(--size-bytes "$SIZE_BYTES")
  fi

  if [[ -n "$OUTPUT" ]]; then
    args+=(--output "$OUTPUT")
  fi
  if (( DRY_RUN )); then
    args+=(--dry-run)
  fi
  if (( FORCE )); then
    args+=(--force)
  fi
  if (( NO_BUILD )); then
    args+=(--no-build)
  fi

  run_cmd "${SCRIPTS_DIR}/create-dev-image.sh" ${args[@]+"${args[@]}"}
}

run_write() {
  local image="${IMAGE:-$DEFAULT_IMAGE}"
  local map_path="${image}.write-map.json"
  local args=(write --disk "$DISK" --map "$map_path")

  [[ -n "$DISK" ]] || fail "write requires --disk"
  [[ -f "$map_path" || "$DRY_RUN" == "1" ]] || fail "write map not found: ${map_path}; run create first"

  if [[ -n "$CONFIRM" ]]; then
    args+=(--confirm "$CONFIRM")
  fi
  if (( DRY_RUN )); then
    args+=(--dry-run)
  fi

  run_ventoyctl "${args[@]}"
}

run_all() {
  OUTPUT="${OUTPUT:-$DEFAULT_IMAGE}"
  run_create
  if [[ -z "$DISK" ]]; then
    select_disk
  fi
  IMAGE="$OUTPUT"
  if (( DRY_RUN )); then
    echo "[flow] dry-run: would write ${IMAGE} to ${DISK}" >&2
    return
  fi
  run_write
}

if (($# > 0)) && [[ "$1" == "-h" || "$1" == "--help" ]]; then
  usage
  exit 0
fi

while (($#)); do
  case "$1" in
    list|build|create|write|all)
      [[ -z "$CMD" ]] || fail "command already set: ${CMD}"
      CMD="$1"
      shift
      ;;
    --disk|--image|--output|--size|--size-bytes|--confirm)
      (($# >= 2)) || fail "$1 requires a value"
      case "$1" in
        --disk) DISK="$2" ;;
        --image) IMAGE="$2" ;;
        --output) OUTPUT="$2" ;;
        --size) SIZE="$2" ;;
        --size-bytes) SIZE_BYTES="$2" ;;
        --confirm) CONFIRM="$2" ;;
      esac
      shift 2
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    --force)
      FORCE=1
      shift
      ;;
    --no-build)
      NO_BUILD=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown argument: $1"
      ;;
  esac
done

case "$CMD" in
  list) run_ventoyctl list-disks ;;
  build) run_cmd "${SCRIPTS_DIR}/build-binaries.sh" ;;
  create) run_create ;;
  write) run_write ;;
  ""|all) run_all ;;
  -h|--help) usage ;;
  *) usage; fail "unknown command: ${CMD}" ;;
esac
