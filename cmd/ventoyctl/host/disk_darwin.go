package host

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"strings"
)

func NormalizeDiskID(value string) (string, error) {
	id := strings.TrimPrefix(strings.TrimPrefix(value, "/dev/"), "r")
	if !strings.HasPrefix(id, "disk") || len(id) == len("disk") {
		return "", fmt.Errorf("disk must look like /dev/diskN or diskN")
	}
	if id == "diskN" {
		return id, nil
	}
	for _, r := range id[len("disk"):] {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("disk must look like /dev/diskN or diskN")
		}
	}
	return id, nil
}

func ListExternalDisks() ([]Disk, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("macOS only")
	}

	out, err := RunPlistJSON("diskutil", "list", "-plist", "external", "physical")
	if err != nil {
		return nil, fmt.Errorf("failed to query disks: %w", err)
	}

	var result diskListResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("failed to parse disk list: %w", err)
	}

	disks := make([]Disk, 0, len(result.WholeDisks))
	for _, id := range result.WholeDisks {
		info, err := readDiskInfo(id)
		if err != nil || !info.WholeDisk {
			continue
		}
		path := info.DeviceNode
		if path == "" {
			path = "/dev/" + info.DeviceIdentifier
		}
		disks = append(disks, Disk{
			ID:        strings.TrimPrefix(path, "/dev/"),
			Path:      path,
			SizeBytes: info.TotalSize,
			SizeHuman: humanBytes(info.TotalSize),
			Name:      firstNonEmpty(info.MediaName, info.VolumeName, info.IORegistryEntryName, "unknown"),
		})
	}
	return disks, nil
}

func SelectDisk(stdin io.Reader, stderr io.Writer, dryRun bool) (string, error) {
	if dryRun {
		fmt.Fprintln(stderr, "[ventoyctl] dry-run: would list disks and ask for disk id")
		return "diskN", nil
	}
	disks, err := ListExternalDisks()
	if err != nil {
		return "", err
	}
	for _, disk := range disks {
		fmt.Fprintf(stderr, "%s\t%s\t%s\n", disk.Path, disk.SizeHuman, disk.Name)
	}
	fmt.Fprint(stderr, "Target disk [diskN]: ")
	var diskID string
	if _, err := fmt.Fscan(stdin, &diskID); err != nil {
		return "", fmt.Errorf("failed to read target disk: %w", err)
	}
	return NormalizeDiskID(diskID)
}

func ValidateTargetDisk(diskID string) (Disk, error) {
	id, err := NormalizeDiskID(diskID)
	if err != nil {
		return Disk{}, err
	}
	info, err := readDiskInfo("/dev/" + id)
	if err != nil {
		return Disk{}, fmt.Errorf("failed to read disk info for /dev/%s: %w", id, err)
	}
	if !info.WholeDisk {
		return Disk{}, fmt.Errorf("/dev/%s is not a whole disk", id)
	}
	if info.Internal {
		return Disk{}, fmt.Errorf("refusing internal disk /dev/%s", id)
	}
	if !info.RemovableMediaOrExternalDevice {
		return Disk{}, fmt.Errorf("/dev/%s is not external/removable", id)
	}
	if info.TotalSize == 0 {
		return Disk{}, fmt.Errorf("failed to read disk size")
	}
	return Disk{
		ID:        id,
		Path:      "/dev/" + id,
		SizeBytes: info.TotalSize,
		SizeHuman: humanBytes(info.TotalSize),
		Name:      firstNonEmpty(info.MediaName, info.VolumeName, info.IORegistryEntryName, "unknown"),
	}, nil
}

func PartitionOffset(path string) (uint64, bool) {
	info, err := readDiskInfo(path)
	if err != nil || info.PartitionMapPartitionOffsetBytes == 0 {
		return 0, false
	}
	return info.PartitionMapPartitionOffsetBytes, true
}

func readDiskInfo(id string) (diskInfo, error) {
	out, err := RunPlistJSON("diskutil", "info", "-plist", id)
	if err != nil {
		return diskInfo{}, err
	}
	var info diskInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return diskInfo{}, err
	}
	return info, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func humanBytes(bytes uint64) string {
	switch {
	case bytes >= 1<<40:
		return fmt.Sprintf("%.2f TiB", float64(bytes)/(1<<40))
	case bytes >= 1<<30:
		return fmt.Sprintf("%.2f GiB", float64(bytes)/(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.2f MiB", float64(bytes)/(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(bytes)/(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
