#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN_PATH="${PROJECT_ROOT}/bin/list-disks"
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
if [[ "${MODE}" == "source" ]]; then
  exec go run ./cmd/list-disks
fi

if [[ -x "${BIN_PATH}" ]]; then
  exec "${BIN_PATH}"
fi

exec go run ./cmd/list-disks
