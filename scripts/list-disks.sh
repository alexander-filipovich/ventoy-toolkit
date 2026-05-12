#!/usr/bin/env bash
set -euo pipefail

# Preflight: this helper is intentionally macOS-only and depends on native disk metadata tools.
if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "Error: scripts/list-disks.sh supports macOS only." >&2
  exit 1
fi

for cmd in diskutil plutil jq; do
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "Error: required command not found: ${cmd}" >&2
    exit 1
  fi
done

# Query only external+physical devices and convert plist output to JSON for stable parsing.
list_json="$(diskutil list -plist external physical 2>/dev/null | plutil -convert json -o - - 2>/dev/null || true)"
if [[ -z "${list_json}" ]]; then
  echo "Error: failed to query external physical disks via diskutil." >&2
  exit 1
fi

mapfile_ids() {
  while IFS= read -r line; do
    [[ -n "${line}" ]] && printf '%s\n' "${line}"
  done
}

identifiers=()
while IFS= read -r id; do
  identifiers+=("${id}")
done < <(printf '%s\n' "${list_json}" | jq -r '.WholeDisks[]? // empty' | mapfile_ids)

if [[ ${#identifiers[@]} -eq 0 ]]; then
  echo "Error: no external physical disks found." >&2
  exit 1
fi

# rows entries use a compact internal format: disk|size_human|size_bytes|name
rows=()
for id in "${identifiers[@]}"; do
  info_json="$(diskutil info -plist "${id}" 2>/dev/null | plutil -convert json -o - - 2>/dev/null || true)"
  [[ -n "${info_json}" ]] || continue

  if ! printf '%s\n' "${info_json}" | jq -e '.WholeDisk == true' >/dev/null 2>&1; then
    continue
  fi

  row="$(printf '%s\n' "${info_json}" | jq -r '
    {
      disk: (.DeviceNode // ("/dev/" + .DeviceIdentifier)),
      size_bytes: (.TotalSize // 0),
      size_human: ((.TotalSize // 0) as $b |
        if $b >= 1000000000 then ((($b / 1000000000) * 10 | floor) / 10 | tostring) + " GB"
        elif $b >= 1000000 then ((($b / 1000000) * 10 | floor) / 10 | tostring) + " MB"
        else ($b|tostring) + " B" end),
      name: (.MediaName // .VolumeName // .IORegistryEntryName // "unknown")
    }
    | "\(.disk)|\(.size_human)|\(.size_bytes)|\(.name)"')"

  rows+=("${row}")
done

if [[ ${#rows[@]} -eq 0 ]]; then
  echo "Error: no usable external physical disks found." >&2
  exit 1
fi

# Human-friendly menu goes to stderr; stdout remains machine-parseable for chaining.
echo "Available external physical disks:" >&2
for i in "${!rows[@]}"; do
  IFS='|' read -r disk size_human _ name <<<"${rows[$i]}"
  printf '%d) %s | %s | %s\n' "$((i + 1))" "${disk}" "${size_human}" "${name}" >&2
done

echo "Select disk number [1-${#rows[@]}]:" >&2
read -r selected_index

if ! [[ "${selected_index}" =~ ^[0-9]+$ ]]; then
  echo "Error: selection must be a number." >&2
  exit 1
fi

if (( selected_index < 1 || selected_index > ${#rows[@]} )); then
  echo "Error: selection out of range." >&2
  exit 1
fi

selected_row="${rows[$((selected_index - 1))]}"
IFS='|' read -r selected_disk selected_human _ selected_name <<<"${selected_row}"
# Output contract: <disk_path><TAB><size_human><TAB><name>
printf '%s\t%s\t%s\n' "${selected_disk}" "${selected_human}" "${selected_name}"
