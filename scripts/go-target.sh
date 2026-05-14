#!/usr/bin/env bash
set -euo pipefail

GO_TARGET_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GO_TARGET_PROJECT_ROOT="$(cd "${GO_TARGET_SCRIPT_DIR}/.." && pwd)"
GO_TARGET_GOCACHE_DEFAULT="${GO_TARGET_PROJECT_ROOT}/.cache/go-build"

run_go_target() {
  local target="${1:?target is required}"
  local mode="${2:-binary}"
  shift 2 || true

  local bin_path="${GO_TARGET_PROJECT_ROOT}/bin/${target}"
  local pkg="./cmd/${target}"
  local go_cache="${GOCACHE:-${GO_TARGET_GOCACHE_DEFAULT}}"

  cd "${GO_TARGET_PROJECT_ROOT}"
  if [[ "${mode}" == "source" ]]; then
    command -v go >/dev/null 2>&1 || {
      echo "Error: go is not installed or not in PATH" >&2
      return 1
    }
    mkdir -p "${go_cache}"
    GOCACHE="${go_cache}" go run "${pkg}" "$@"
    return
  fi

  if [[ -x "${bin_path}" ]]; then
    "${bin_path}" "$@"
    return
  fi

  command -v go >/dev/null 2>&1 || {
    echo "Error: ${bin_path} not found and go is unavailable for fallback" >&2
    return 1
  }
  mkdir -p "${go_cache}"
  GOCACHE="${go_cache}" go run "${pkg}" "$@"
}
