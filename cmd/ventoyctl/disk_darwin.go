package main

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
)

type diskListResult struct {
	WholeDisks []string `json:"WholeDisks"`
}

type diskInfo struct {
	DeviceNode                       string `json:"DeviceNode"`
	DeviceIdentifier                 string `json:"DeviceIdentifier"`
	TotalSize                        uint64 `json:"TotalSize"`
	MediaName                        string `json:"MediaName"`
	VolumeName                       string `json:"VolumeName"`
	IORegistryEntryName              string `json:"IORegistryEntryName"`
	WholeDisk                        bool   `json:"WholeDisk"`
	Internal                         bool   `json:"Internal"`
	RemovableMediaOrExternalDevice   bool   `json:"RemovableMediaOrExternalDevice"`
	PartitionMapPartitionOffsetBytes uint64 `json:"PartitionMapPartitionOffset"`
}

func normalizeDiskID(value string) (string, error) {
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

	out, err := runPlistJSON("diskutil", "list", "-plist", "external", "physical")
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

func ValidateTargetDisk(diskID string) (Disk, error) {
	id, err := normalizeDiskID(diskID)
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

func partitionOffset(path string) (uint64, bool) {
	info, err := readDiskInfo(path)
	if err != nil || info.PartitionMapPartitionOffsetBytes == 0 {
		return 0, false
	}
	return info.PartitionMapPartitionOffsetBytes, true
}

func readDiskInfo(id string) (diskInfo, error) {
	out, err := runPlistJSON("diskutil", "info", "-plist", id)
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
