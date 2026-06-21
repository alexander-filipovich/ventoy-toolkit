#!/usr/bin/env bash

run_cmd() {
  "$@"
}

run_ventoyctl() {
  local root="${PROJECT_ROOT:?PROJECT_ROOT is required}"
  local bin="${root}/bin/ventoyctl"
  if [[ -x "${bin}" ]]; then
    "${bin}" "$@"
    return
  fi
  (
    cd "${root}"
    GOCACHE="${GOCACHE:-${root}/.cache/go-build}" \
      GOMODCACHE="${GOMODCACHE:-${root}/.cache/go-mod}" \
      go run ./cmd/ventoyctl "$@"
  )
}
