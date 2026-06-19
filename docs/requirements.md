# Requirements

## Supported Platform
- macOS host (Darwin)

If sparse extent detection is unavailable, `image-extents` falls back to a
full-image range.

## Host Requirements
- Built-in macOS tools:
  - `diskutil`
  - `plutil`

## Build Requirements
- Go 1.24+ toolchain in `PATH`
- `golang.org/x/sys/unix` for platform-provided `SEEK_DATA` and `SEEK_HOLE`

## Container Requirements
- Docker Desktop
- `docker compose`
- Ventoy version is pinned in `docker/Dockerfile`
- Ventoy tooling container runs as `linux/amd64`
