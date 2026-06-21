#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="${PROJECT_ROOT}/scripts"
DEFAULT_IMAGE="./artifacts/ventoy-dev.img"

CMD="" DISK="" IMAGE="" OUTPUT="" SIZE="" SIZE_BYTES="" CONFIRM="" DRY_RUN=0

fail() { echo "[flow][error] $*" >&2; exit 1; }

usage() {
  cat >&2 <<'EOF'
Usage:
  ./ventoy-flow.sh [--disk diskN] [--confirm diskN] [--dry-run]
  ./ventoy-flow.sh list
  ./ventoy-flow.sh create [--size 128m|--size-bytes N] [--output PATH] [--dry-run]
  ./ventoy-flow.sh write --disk diskN [--image PATH] [--confirm diskN] [--dry-run]
  ./ventoy-flow.sh all [--disk diskN] [--output PATH] [--confirm diskN] [--dry-run]
EOF
}

go_ventoyctl() {
  (
    cd "${PROJECT_ROOT}"
    GOCACHE="${GOCACHE:-${PROJECT_ROOT}/.cache/go-build}" go run ./cmd/ventoyctl "$@"
  )
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
  if (( DRY_RUN )); then
    echo "[flow] dry-run: would list disks and ask for disk id" >&2
    DISK="diskN"
    return
  fi
  go_ventoyctl list-disks >&2
  printf 'Target disk [diskN]: ' >&2
  read -r DISK
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

  "${SCRIPTS_DIR}/create-dev-image.sh" "${args[@]}"
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

  go_ventoyctl "${args[@]}"
}

run_all() {
  OUTPUT="${OUTPUT:-$DEFAULT_IMAGE}"
  run_create
  if [[ -z "$DISK" ]]; then
    select_disk
  fi
  IMAGE="$OUTPUT"
  run_write
}

if (($# > 0)) && [[ "$1" == "-h" || "$1" == "--help" ]]; then
  usage
  exit 0
fi

while (($#)); do
  case "$1" in
    list|create|write|all)
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
  list) go_ventoyctl list-disks ;;
  create) run_create ;;
  write) run_write ;;
  ""|all) run_all ;;
  -h|--help) usage ;;
  *) usage; fail "unknown command: ${CMD}" ;;
esac
