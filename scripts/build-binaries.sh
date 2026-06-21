#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

mkdir -p "${PROJECT_ROOT}/bin"
(
  cd "${PROJECT_ROOT}"
  GOCACHE="${GOCACHE:-${PROJECT_ROOT}/.cache/go-build}" \
    GOMODCACHE="${GOMODCACHE:-${PROJECT_ROOT}/.cache/go-mod}" \
    go build -o "${PROJECT_ROOT}/bin/ventoyctl" ./cmd/ventoyctl
)
echo "[build-binaries] wrote ${PROJECT_ROOT}/bin/ventoyctl" >&2
