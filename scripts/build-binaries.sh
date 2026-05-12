#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN_DIR="${PROJECT_ROOT}/bin"

log() {
  printf '[build] %s\n' "$*" >&2
}

fail() {
  printf '[build][error] %s\n' "$*" >&2
  exit 1
}

step() {
  printf '\n[build][step] %s\n' "$*" >&2
}

if ! command -v go >/dev/null 2>&1; then
  fail "go is not installed or not in PATH"
fi

step "Discover build targets in cmd/"
targets=()
while IFS= read -r target; do
  [[ -n "${target}" ]] || continue
  targets+=("${target}")
done < <(find "${PROJECT_ROOT}/cmd" -mindepth 1 -maxdepth 1 -type d -print | sort)

if [[ ${#targets[@]} -eq 0 ]]; then
  fail "no cmd targets found under ${PROJECT_ROOT}/cmd"
fi

log "Found ${#targets[@]} target(s)"
for target in "${targets[@]}"; do
  log "- ./cmd/$(basename "${target}")"
done

step "Build binaries"
mkdir -p "${BIN_DIR}"
log "Output directory: ${BIN_DIR}"

built=0
for target in "${targets[@]}"; do
  name="$(basename "${target}")"
  pkg="./cmd/${name}"
  out="${BIN_DIR}/${name}"

  log "Building ${pkg}"
  (
    cd "${PROJECT_ROOT}"
    go build -o "${out}" "${pkg}"
  )
  log "Built -> ${out}"
  built=$((built + 1))
done

step "Summary"
log "Built ${built} binary(ies) successfully"
