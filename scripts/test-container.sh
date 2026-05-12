#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${PROJECT_ROOT}/docker/compose.yaml"

if ! command -v docker >/dev/null 2>&1; then
  echo "Error: docker is not installed or not in PATH." >&2
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "Error: docker compose is not available. Install Docker Desktop (or compose plugin)." >&2
  exit 1
fi

echo "Building container image (if needed)..."
docker compose -f "${COMPOSE_FILE}" build ventoy

echo "Running Ventoy CLI help..."
output="$(docker compose -f "${COMPOSE_FILE}" run --rm ventoy --help 2>&1)"
echo "${output}"

if ! grep -Eiq 'usage|ventoy|options' <<<"${output}"; then
  echo "Error: Ventoy help output did not match expected content." >&2
  exit 1
fi

echo "Container smoke test passed."
