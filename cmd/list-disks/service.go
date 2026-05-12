package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type listResult struct {
	WholeDisks []string `json:"WholeDisks"`
}

type diskInfo struct {
	DeviceNode          string `json:"DeviceNode"`
	DeviceIdentifier    string `json:"DeviceIdentifier"`
	TotalSize           uint64 `json:"TotalSize"`
	MediaName           string `json:"MediaName"`
	VolumeName          string `json:"VolumeName"`
	IORegistryEntryName string `json:"IORegistryEntryName"`
	WholeDisk           bool   `json:"WholeDisk"`
}

type Candidate struct {
	DiskPath  string
	SizeHuman string
	Name      string
}

func listExternalPhysicalCandidates() ([]Candidate, error) {
	ids, err := listExternalPhysicalDisks()
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no external physical disks found")
	}

	candidates, err := buildCandidates(ids)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no usable external physical disks found")
	}

	return candidates, nil
}

func listExternalPhysicalDisks() ([]string, error) {
	out, err := runPlistAsJSON("diskutil", "list", "-plist", "external", "physical")
	if err != nil {
		return nil, fmt.Errorf("failed to query disks: %w", err)
	}

	var res listResult
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, fmt.Errorf("failed to parse disk list json: %w", err)
	}

	return res.WholeDisks, nil
}

func buildCandidates(ids []string) ([]Candidate, error) {
	candidates := make([]Candidate, 0, len(ids))

	for _, id := range ids {
		out, err := runPlistAsJSON("diskutil", "info", "-plist", id)
		if err != nil {
			continue
		}

		var info diskInfo
		if err := json.Unmarshal(out, &info); err != nil {
			continue
		}
		if !info.WholeDisk {
			continue
		}

		diskPath := info.DeviceNode
		if diskPath == "" && info.DeviceIdentifier != "" {
			diskPath = "/dev/" + info.DeviceIdentifier
		}
		if diskPath == "" {
			continue
		}

		name := firstNonEmpty(info.MediaName, info.VolumeName, info.IORegistryEntryName, "unknown")
		candidates = append(candidates, Candidate{
			DiskPath:  diskPath,
			SizeHuman: bytesToHuman(info.TotalSize),
			Name:      name,
		})
	}

	return candidates, nil
}

func promptSelection(candidates []Candidate) (Candidate, error) {
	fmt.Fprintln(os.Stderr, "Available external physical disks:")
	for i, c := range candidates {
		fmt.Fprintf(os.Stderr, "%d) %s | %s | %s\n", i+1, c.DiskPath, c.SizeHuman, c.Name)
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprintf(os.Stderr, "Select disk number [1-%d]:\n", len(candidates))

		input, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return Candidate{}, fmt.Errorf("failed to read selection: %w", err)
		}

		input = strings.TrimSpace(input)
		idx, convErr := strconv.Atoi(input)
		if convErr != nil || idx < 1 || idx > len(candidates) {
			if convErr != nil {
				fmt.Fprintln(os.Stderr, "Invalid input: enter a number.")
			} else {
				fmt.Fprintln(os.Stderr, "Invalid input: selection out of range.")
			}
			if errors.Is(err, io.EOF) {
				return Candidate{}, errors.New("selection cancelled")
			}
			continue
		}

		return candidates[idx-1], nil
	}
}

func bytesToHuman(b uint64) string {
	if b >= 1_000_000_000 {
		return fmt.Sprintf("%.1f GB", float64(b)/1_000_000_000)
	}
	if b >= 1_000_000 {
		return fmt.Sprintf("%.1f MB", float64(b)/1_000_000)
	}
	return fmt.Sprintf("%d B", b)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
