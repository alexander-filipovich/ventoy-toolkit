#!/usr/bin/env bash
set -euo pipefail

# This helper is intentionally macOS-only.
if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "Error: macOS only." >&2
  exit 1
fi

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Error: missing ${cmd}" >&2
    exit 1
  fi
}

require_cmd diskutil
require_cmd plutil
require_cmd jq

# Convert diskutil plist output to JSON so we can parse it predictably.
plist_to_json() {
  plutil -convert json -o - - 2>/dev/null
}

bytes_to_human() {
  local bytes="$1"

  if (( bytes >= 1000000000 )); then
    awk -v b="$bytes" 'BEGIN { printf "%.1f GB", b/1000000000 }'
    return
  fi

  if (( bytes >= 1000000 )); then
    awk -v b="$bytes" 'BEGIN { printf "%.1f MB", b/1000000 }'
    return
  fi

  printf '%s B' "$bytes"
}

list_json="$(diskutil list -plist external physical 2>/dev/null | plist_to_json || true)"
if [[ -z "$list_json" ]]; then
  echo "Error: failed to query disks." >&2
  exit 1
fi

mapfile_ids() {
  while IFS= read -r id; do
    [[ -n "$id" ]] && printf '%s\n' "$id"
  done
}

identifiers=()
while IFS= read -r id; do
  identifiers+=("$id")
done < <(printf '%s\n' "$list_json" | jq -r '.WholeDisks[]? // empty' | mapfile_ids)

if [[ ${#identifiers[@]} -eq 0 ]]; then
  echo "Error: no external physical disks found." >&2
  exit 1
fi

declare -a rows
for id in "${identifiers[@]}"; do
  info_json="$(diskutil info -plist "$id" 2>/dev/null | plist_to_json || true)"
  [[ -n "$info_json" ]] || continue

  whole_disk="$(printf '%s\n' "$info_json" | jq -r '.WholeDisk // false')"
  [[ "$whole_disk" == "true" ]] || continue

  disk_path="$(printf '%s\n' "$info_json" | jq -r '.DeviceNode // ("/dev/" + .DeviceIdentifier)')"
  size_bytes="$(printf '%s\n' "$info_json" | jq -r '.TotalSize // 0')"
  name="$(printf '%s\n' "$info_json" | jq -r '.MediaName // .VolumeName // .IORegistryEntryName // "unknown"')"
  size_human="$(bytes_to_human "$size_bytes")"

  rows+=("${disk_path}|${size_human}|${name}")
done

count="${#rows[@]}"
if (( count == 0 )); then
  echo "Error: no usable external physical disks found." >&2
  exit 1
fi

# UI goes to stderr; stdout is machine-readable for follow-up scripts.
echo "Available external physical disks:" >&2
for i in "${!rows[@]}"; do
  IFS='|' read -r disk size_human name <<<"${rows[$i]}"
  printf '%d) %s | %s | %s\n' "$((i + 1))" "$disk" "$size_human" "$name" >&2
done

echo "Select disk number [1-${count}]:" >&2
read -r selected

if ! [[ "$selected" =~ ^[0-9]+$ ]]; then
  echo "Error: selection must be a number." >&2
  exit 1
fi

if (( selected < 1 || selected > count )); then
  echo "Error: selection out of range." >&2
  exit 1
fi

selected_row="${rows[$((selected - 1))]}"
IFS='|' read -r selected_disk selected_human selected_name <<<"$selected_row"

# Output contract: <disk_path><TAB><size_human><TAB><name>
printf '%s\t%s\t%s\n' "$selected_disk" "$selected_human" "$selected_name"
