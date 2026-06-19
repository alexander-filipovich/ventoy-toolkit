#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="${SCRIPT_DIR}"
SCRIPTS_DIR="${PROJECT_ROOT}/scripts"
ARTIFACTS_DIR="${PROJECT_ROOT}/artifacts"
SESSION_FILE="${ARTIFACTS_DIR}/.flow-session.env"
source "${SCRIPTS_DIR}/go-target.sh"

DEFAULT_OUTPUT="./artifacts/ventoy-dev.img"

CMD=""
SESSION_CMD=""
DISK=""
IMAGE=""
OUTPUT=""
SIZE=""
SIZE_BYTES=""
CONFIRM=""
DRY_RUN=0

SESSION_ID=""
SESSION_DISK=""
SESSION_IMAGE=""
SESSION_SIZE_BYTES=""

REMOVE_SESSION_ON_EXIT=0

log() { echo "[flow] $*" >&2; }
fail() { echo "[flow][error] $*" >&2; exit 1; }

usage() {
  cat >&2 <<'EOF_USAGE'
Usage:
  ./ventoy-flow.sh select [--dry-run]
  ./ventoy-flow.sh create [--size 128m | --size-bytes N] [--output PATH] [--dry-run]
  ./ventoy-flow.sh parse [--image PATH] [--dry-run]
  ./ventoy-flow.sh write [--disk /dev/diskN] [--image PATH] [--confirm diskN] [--dry-run]
  ./ventoy-flow.sh all [--output PATH] [--confirm diskN] [--dry-run]
  ./ventoy-flow.sh session show|reset

Options:
  --disk /dev/diskN|diskN
  --image PATH
  --output PATH
  --size VALUE          Exact image size with suffix: Ng, Nm, Nk, Nb (e.g. 128m)
  --size-bytes N        Exact image size in bytes
  --confirm diskN
  --dry-run
  -h, --help
EOF_USAGE
}

new_session_id() {
  printf '%s-%s-%s' "$(date +%s)" "$$" "$RANDOM"
}

session_get() {
  local key="$1"
  [[ -f "$SESSION_FILE" ]] || return 0
  awk -F= -v k="$key" '$1==k {v=$2; gsub(/^\047|\047$/, "", v); print v}' "$SESSION_FILE" 2>/dev/null || true
}

normalize_disk() {
  local d="$1"
  d="${d##*/}"
  d="${d#r}"
  [[ "$d" =~ ^disk[0-9]+$ ]] || fail "disk must look like /dev/diskN or diskN"
  printf '/dev/%s' "$d"
}

parse_size_to_bytes() {
  local v="$1"
  local lower num suffix mult
  lower="$(printf '%s' "$v" | tr '[:upper:]' '[:lower:]')"
  if [[ "$lower" =~ ^([0-9]+)([gmkb])$ ]]; then
    num="${BASH_REMATCH[1]}"
    suffix="${BASH_REMATCH[2]}"
  else
    fail "invalid --size '${v}' (examples: 12g, 1024m, 4096k, 1048576b)"
  fi

  case "$suffix" in
    g) mult=$((1024 * 1024 * 1024)) ;;
    m) mult=$((1024 * 1024)) ;;
    k) mult=1024 ;;
    b) mult=1 ;;
    *) fail "invalid --size suffix in '${v}'" ;;
  esac

  printf '%s' "$((num * mult))"
}

validate_target_disk() {
  local disk_node="$1"
  local info_plist whole_disk internal_disk external_disk

  command -v diskutil >/dev/null 2>&1 || fail "diskutil not found"
  command -v plutil >/dev/null 2>&1 || fail "plutil not found"

  info_plist="$(diskutil info -plist "$disk_node" 2>/dev/null || true)"
  [[ -n "$info_plist" ]] || fail "failed to read disk info for ${disk_node}"

  whole_disk="$(printf '%s' "$info_plist" | plutil -extract WholeDisk raw - 2>/dev/null || true)"
  internal_disk="$(printf '%s' "$info_plist" | plutil -extract Internal raw - 2>/dev/null || true)"
  external_disk="$(printf '%s' "$info_plist" | plutil -extract RemovableMediaOrExternalDevice raw - 2>/dev/null || true)"

  [[ "$whole_disk" == "true" ]] || fail "${disk_node} is not a whole disk"
  [[ "$internal_disk" == "false" ]] || fail "refusing internal disk ${disk_node}"
  [[ "$external_disk" == "true" ]] || fail "${disk_node} is not external/removable"
}

ensure_session_loaded() {
  mkdir -p "$ARTIFACTS_DIR"

  SESSION_ID="$(session_get SESSION_ID)"
  SESSION_DISK="$(session_get DISK)"
  SESSION_IMAGE="$(session_get IMAGE)"
  SESSION_SIZE_BYTES="$(session_get SIZE_BYTES)"

  if [[ -z "$SESSION_ID" ]]; then
    SESSION_ID="$(new_session_id)"
    SESSION_DISK=""
    SESSION_IMAGE=""
    SESSION_SIZE_BYTES=""
  fi

}

save_session() {
  mkdir -p "$ARTIFACTS_DIR"
  cat > "$SESSION_FILE" <<EOF_SESSION
SESSION_ID='${SESSION_ID}'
DISK='${SESSION_DISK}'
IMAGE='${SESSION_IMAGE}'
SIZE_BYTES='${SESSION_SIZE_BYTES}'
EOF_SESSION
}

reset_session_file() {
  rm -f "$SESSION_FILE"
}

cleanup_session() {
  if (( REMOVE_SESSION_ON_EXIT == 0 )); then
    return
  fi
  if [[ ! -f "$SESSION_FILE" ]]; then
    return
  fi

  rm -f "$SESSION_FILE"
  log "session reset"
}

on_exit() {
  local ec=$?
  cleanup_session
  exit "$ec"
}

run_select() {
  if (( DRY_RUN )); then
    log "dry-run: ${SCRIPTS_DIR}/list-disks.sh"
    log "dry-run: would save selected disk to session"
    return
  fi

  local selected
  selected="$("${SCRIPTS_DIR}/list-disks.sh")"
  [[ -n "$selected" ]] || fail "selector returned empty disk"
  SESSION_DISK="$(normalize_disk "$selected")"
  validate_target_disk "$SESSION_DISK"
  save_session
  log "selected_disk=${SESSION_DISK}"
}

run_create() {
  local effective_size_bytes=""
  local has_size_source=0
  local create_args=()

  [[ -z "$DISK" ]] || fail "--disk is used by select/write, not create"

  if [[ -n "$SIZE" && -n "$SIZE_BYTES" ]]; then
    fail "use only one explicit size flag: --size OR --size-bytes"
  fi

  if [[ -n "$SIZE" ]]; then
    effective_size_bytes="$(parse_size_to_bytes "$SIZE")"
    has_size_source=1
  elif [[ -n "$SIZE_BYTES" ]]; then
    [[ "$SIZE_BYTES" =~ ^[0-9]+$ ]] || fail "--size-bytes must be an unsigned integer"
    effective_size_bytes="$SIZE_BYTES"
    has_size_source=1
  fi

  if (( has_size_source == 1 )); then
    create_args+=(--size-bytes "$effective_size_bytes")
    SESSION_SIZE_BYTES="$effective_size_bytes"
  else
    log "create size not set, using create-dev-image default"
    SESSION_SIZE_BYTES=""
  fi

  if [[ -n "$OUTPUT" ]]; then
    create_args+=(--output "$OUTPUT")
    SESSION_IMAGE="$OUTPUT"
  else
    SESSION_IMAGE="$DEFAULT_OUTPUT"
  fi

  if (( DRY_RUN )); then
    create_args+=(--dry-run)
  fi

  save_session
  if ((${#create_args[@]})); then
    "${SCRIPTS_DIR}/create-dev-image.sh" "${create_args[@]}"
  else
    "${SCRIPTS_DIR}/create-dev-image.sh"
  fi
}

run_write() {
  local write_disk=""
  local write_image=""
  local write_args=()

  if [[ -n "$DISK" ]]; then
    write_disk="$(normalize_disk "$DISK")"
  elif [[ -n "$SESSION_DISK" ]]; then
    write_disk="$SESSION_DISK"
  else
    log "disk not set, running select"
    run_select
    write_disk="$SESSION_DISK"
  fi

  if [[ -n "$IMAGE" ]]; then
    write_image="$IMAGE"
  elif [[ -n "$SESSION_IMAGE" ]]; then
    write_image="$SESSION_IMAGE"
  else
    fail "write needs --image or a session image; run create first or pass --image"
  fi

  write_args+=(--disk "$write_disk" --image "$write_image")
  if [[ -n "$CONFIRM" ]]; then
    write_args+=(--confirm "$CONFIRM")
  fi
  if (( DRY_RUN )); then
    write_args+=(--dry-run)
  fi

  SESSION_DISK="$write_disk"
  SESSION_IMAGE="$write_image"
  save_session
  "${SCRIPTS_DIR}/write-image.sh" "${write_args[@]}"
}

run_parse() {
  local parse_image=""

  if [[ -n "$IMAGE" ]]; then
    parse_image="$IMAGE"
  elif [[ -n "$SESSION_IMAGE" ]]; then
    parse_image="$SESSION_IMAGE"
  else
    parse_image="$DEFAULT_OUTPUT"
  fi

  if (( DRY_RUN )); then
    log "dry-run: image-extents --image ${parse_image}"
    return
  fi

  [[ -f "$parse_image" ]] || fail "image not found: ${parse_image}"
  run_go_target image-extents "${GO_TOOL_MODE:-binary}" \
    --image "$parse_image" \
    --image-path "$parse_image"
}

run_all() {
  run_select
  run_create
  IMAGE="$SESSION_IMAGE"
  run_write
  REMOVE_SESSION_ON_EXIT=1
}

if (($# == 0)); then
  usage
  exit 1
fi

if [[ "${1-}" == "-h" || "${1-}" == "--help" ]]; then
  usage
  exit 0
fi

CMD="$1"
shift

if [[ "$CMD" == "session" ]]; then
  (($# >= 1)) || fail "session requires subcommand: show|reset"
  SESSION_CMD="$1"
  shift
fi

while (($#)); do
  case "$1" in
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

trap on_exit EXIT INT TERM
ensure_session_loaded

case "$CMD" in
  select)
    run_select
    ;;
  create)
    run_create
    ;;
  parse)
    run_parse
    ;;
  write)
    run_write
    ;;
  all)
    run_all
    ;;
  session)
    case "$SESSION_CMD" in
      show)
        if [[ -f "$SESSION_FILE" ]]; then
          cat "$SESSION_FILE"
        else
          log "session is empty"
        fi
        ;;
      reset)
        reset_session_file
        log "session reset"
        ;;
      *)
        fail "unknown session subcommand: ${SESSION_CMD}"
        ;;
    esac
    ;;
  *)
    usage
    fail "unknown command: ${CMD}"
    ;;
esac
