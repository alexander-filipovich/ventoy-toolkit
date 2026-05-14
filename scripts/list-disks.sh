#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
source "${SCRIPT_DIR}/go-target.sh"
MODE="binary"

usage() {
  cat >&2 <<'EOF'
Usage: scripts/list-disks.sh [--source]
  --source  Force run from Go source (go run ./cmd/list-disks)
EOF
}

case "${1-}" in
  "") ;;
  --source) MODE="source" ;;
  -h|--help) usage; exit 0 ;;
  *) usage; echo "Error: unknown argument: ${1}" >&2; exit 1 ;;
esac
[[ $# -le 1 ]] || { usage; echo "Error: too many arguments" >&2; exit 1; }

cd "${PROJECT_ROOT}"
selected_line="$(run_go_target "list-disks" "${MODE}")"
selected_disk="$(printf '%s' "${selected_line}" | tr -d '\r' | sed -e 's/[[:space:]]*$//')"
[[ "${selected_disk}" =~ ^/dev/disk[0-9]+$ ]] || { echo "Error: selector returned unexpected value: ${selected_disk}" >&2; exit 1; }
printf '%s\n' "${selected_disk}"
