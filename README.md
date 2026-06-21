# Ventoy Toolkit — macOS Ventoy USB Writer

Ventoy Toolkit is a small, auditable CLI tool for creating a Ventoy USB drive on macOS. It is a Ventoy installer for Mac that avoids passing the physical USB drive into Docker and avoids reimplementing Ventoy's full GPT/boot layout from scratch.

The core idea is an **MBR transplant** flow: use the official Ventoy installer to create a small reference image, then copy only the required boot/service ranges to the real USB drive on macOS. The target disk is written directly by the macOS host, not by a Linux container.

Ventoy Toolkit is not affiliated with the official Ventoy project.

## Why This Exists

[Ventoy](https://www.ventoy.net/) is one of the best tools for building a bootable USB drive that can hold many ISO files. Install Ventoy once, copy ISOs onto the first partition, and boot from a menu.

The problem: official Ventoy install tooling is Windows/Linux-first. On macOS, common workarounds are:

- booting a Ventoy LiveCD or Linux environment;
- using a Linux VM with USB passthrough;
- routing a physical USB drive through Docker/NBD/QEMU;
- using a direct macOS layout port that recreates Ventoy's disk layout itself.

Ventoy Toolkit takes a narrower path. It uses official Ventoy output as the source of truth, then performs a small, inspectable macOS write operation.

## Ventoy Layout Contract

Ventoy Toolkit is built around Ventoy's public high-level MBR layout contract, documented in [Ventoy Disk Layout In MBR](https://www.ventoy.net/en/doc_disk_layout.html).

The relevant contract is simple:

- Ventoy's MBR layout has two partitions.
- Part 1 is the user data partition, exFAT by default, used to hold ISO files.
- Part 2 is the small Ventoy EFI/service partition.
- The 1 MB gap before Part 1 is used for the Legacy BIOS bootloader.
- MBR is used to support Legacy BIOS boot.

```text
┌────────────────────────────────────────────┐
│ MBR + 1 MB gap                             │  Legacy BIOS bootloader area
├────────────────────────────────────────────┤
│ Part 1                                     │  Main partition for ISO files
│ exFAT by default                           │
│ reformat-capable in Ventoy                 │
├────────────────────────────────────────────┤
│ Part 2                                     │  Small FAT EFI/service partition
│ VTOYEFI                                    │  Ventoy boot files
└────────────────────────────────────────────┘
```

That distinction matters. Ventoy Toolkit does not try to recreate every low-level Ventoy artifact from scratch. It uses official Ventoy output for the reference image, then relies on this documented two-partition MBR structure to transplant the minimum required ranges to the real USB drive.

## How It Works

1. A Ventoy reference image is either bundled in `artifacts/` or generated locally.
2. If generation is needed, Docker runs the official Ventoy installer against that image file.
3. `ventoyctl` parses the reference image layout and creates a write map.
4. The writer patches the MBR partition entries for the real target disk size.
5. The writer copies only the required ranges:
   - patched MBR;
   - pre-partition boot area;
   - Ventoy EFI/service partition.
6. macOS formats the user data partition as ExFAT.
7. The target MBR is verified against the patched reference MBR.

This means Docker may be used to generate a reference image, but Docker never receives the real USB device.

## Quick Start

> **Warning:** writing a Ventoy USB drive is destructive. Double-check the target disk.

List external physical disks:

```sh
./ventoy-flow.sh list
```

Preview the full flow without writing:

```sh
./ventoy-flow.sh --dry-run --confirm diskN
```

Run the default flow:

```sh
./ventoy-flow.sh
```

Or run the steps separately:

```sh
./ventoy-flow.sh create
./ventoy-flow.sh write --disk diskN --confirm diskN
```

Build the Go CLI binary:

```sh
./ventoy-flow.sh build
```

Run from Go source instead of building or using `bin/ventoyctl`:

```sh
./ventoy-flow.sh --no-build
```

## Release Artifacts and Checksums

Release builds may include baked artifacts for users who do not want to install Docker or Go:

- `bin/ventoyctl`
- `bin/ventoyctl.sha256`
- `artifacts/ventoy-dev.img`
- `artifacts/ventoy-dev.img.sha256`
- `artifacts/ventoy-dev.img.write-map.json`
- `artifacts/ventoy-dev.img.write-map.json.sha256`

Missing checksums mean the cache is invalid. Ventoy Toolkit does not silently trust cached binaries or images without matching `.sha256` files.

Default behavior:

- if `bin/ventoyctl` and `bin/ventoyctl.sha256` are valid, the binary is used;
- if the binary is missing or the checksum does not match, `ventoyctl` is rebuilt;
- if cached image artifacts and checksums are valid, image generation is skipped;
- if image cache is invalid, Docker regenerates the reference image;
- `--force` regenerates the reference image cache;
- `--no-build` runs `go run ./cmd/ventoyctl` instead of building or using `bin/ventoyctl`.

## Commands

High-level wrapper:

```sh
./ventoy-flow.sh [--disk diskN] [--confirm diskN] [--no-build] [--dry-run]
./ventoy-flow.sh list
./ventoy-flow.sh build
./ventoy-flow.sh create [--size 128m|--size-bytes N] [--output PATH] [--force] [--no-build] [--dry-run]
./ventoy-flow.sh write --disk diskN [--image PATH] [--confirm diskN] [--no-build] [--dry-run]
./ventoy-flow.sh all [--disk diskN] [--output PATH] [--confirm diskN] [--force] [--no-build] [--dry-run]
```

Lower-level Go CLI:

```sh
ventoyctl list-disks
ventoyctl select-disk [--dry-run]
ventoyctl map-image --image PATH --image-path PATH --partition-json PATH
ventoyctl write --map PATH --disk diskN [--confirm diskN] [--dry-run]
```

## Safety Model

Ventoy Toolkit is intentionally conservative:

- macOS host only;
- external physical disks only;
- whole-disk targets only, not mounted volumes;
- explicit confirmation token required for real writes;
- dry-run mode available;
- real writes require root privileges;
- cached binaries and images require separate `.sha256` files;
- post-format MBR verification compares the target disk MBR with the patched reference MBR.

The write operation is still destructive. If you confirm the wrong disk, data on that disk can be lost.

## Comparison with Alternatives

| Tool | Approach | Strength | Tradeoff |
| --- | --- | --- | --- |
| [Official Ventoy](https://www.ventoy.net/) | Native installer for Windows/Linux | Canonical behavior and broad feature support | No native macOS installer flow |
| [VentoyDocker](https://github.com/garybowers/iventoy_docker) | Runs official Ventoy in Docker and exposes the USB through NBD/QEMU | Keeps official Ventoy writing the disk | Complex I/O path: macOS raw disk → NBD/QEMU → Docker |
| [ventoy-macos-install](https://gist.github.com/VladimirMakaev/93503ab7c63c7bf4b0cada5db726614a) | Direct macOS proof-of-concept that writes layout, boot code, and EFI image | Small terminal PoC without Docker/VM | Reimplements Ventoy layout details directly |
| [Mactoy](https://github.com/cashcon57/mactoy) | Polished native macOS app built around the direct-layout lineage | Best GUI/end-user product | Larger product surface: GUI, helper, permissions, layout implementation |
| Ventoy Toolkit | Official reference image + small MBR transplant writer | Small auditable CLI/core with fewer layout assumptions | MBR-only, no GUI, no update-in-place |

The important distinction is responsibility. Ventoy Toolkit tries to minimize how much it must know about Ventoy internals. It does not build a full GPT writer, GUI, privileged helper, ISO manager, updater, or raw image flasher. It maps official Ventoy output, patches the target MBR, copies the minimum required ranges, formats the data partition, and verifies the result.

These tools serve different layers of the same problem space. Ventoy Toolkit is aimed at users and developers who want a smaller, composable, auditable installer core rather than a full end-user application.

## Requirements

For release artifacts:

- macOS;
- `diskutil`;
- `shasum` or `sha256sum`;
- root privileges for real writes.

For regenerating the reference image:

- Docker Desktop;
- `docker compose`.

For source mode or rebuilding `ventoyctl`:

- Go 1.24+.

## Current Limitations

- MBR-only.
- No GUI.
- No signed or notarized macOS app.
- No Ventoy update-in-place flow.
- Docker is still required when regenerating the reference image.
- Go is still required for `--no-build` source mode or rebuilding binaries.

## License

MIT. See [LICENSE](LICENSE).
