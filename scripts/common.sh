#!/usr/bin/env bash

run_cmd() {
  "$@"
}

file_sha256() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    echo "sha256 tool not found" >&2
    return 1
  fi
}

write_checksum() {
  local path="$1"
  file_sha256 "${path}" > "${path}.sha256"
}

verify_checksum() {
  local path="$1"
  local sum_path="${path}.sha256"
  [[ -f "${path}" && -f "${sum_path}" ]] || return 1
  [[ "$(cat "${sum_path}")" == "$(file_sha256 "${path}")" ]]
}

ensure_ventoyctl() {
  local root="${PROJECT_ROOT:?PROJECT_ROOT is required}"
  local bin="${root}/bin/ventoyctl"
  if [[ -x "${bin}" ]] && verify_checksum "${bin}"; then
    return
  fi
  echo "[ventoyctl] building ${bin}" >&2
  run_cmd "${root}/scripts/build-binaries.sh" || return
}

run_ventoyctl_source() {
  local root="${PROJECT_ROOT:?PROJECT_ROOT is required}"
  (
    cd "${root}"
    GOCACHE="${GOCACHE:-${root}/.cache/go-build}" \
      GOMODCACHE="${GOMODCACHE:-${root}/.cache/go-mod}" \
      go run ./cmd/ventoyctl "$@"
  )
}

run_ventoyctl() {
  local root="${PROJECT_ROOT:?PROJECT_ROOT is required}"
  local bin="${root}/bin/ventoyctl"
  if [[ "${NO_BUILD:-0}" == "1" ]]; then
    run_ventoyctl_source "$@"
    return
  fi
  ensure_ventoyctl || return
  "${bin}" "$@"
}
