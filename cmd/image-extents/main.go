package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"syscall"
)

const (
	seekData = 3
	seekHole = 4
)

type byteRange struct {
	OffsetBytes uint64 `json:"offset_bytes"`
	LengthBytes uint64 `json:"length_bytes"`
}

type taggedRange struct {
	Tag string `json:"tag"`
	byteRange
}

type extentsOutput struct {
	Status        string        `json:"status"`
	ExtentsStatus string        `json:"extents_status"`
	LogicalBytes  uint64        `json:"logical_bytes"`
	Source        string        `json:"source"`
	Extents       []byteRange   `json:"extents"`
	RawExtents    []byteRange   `json:"raw_extents"`
	CuratedRanges []taggedRange `json:"curated_ranges,omitempty"`
	WriterPolicy  writerPolicy  `json:"writer_policy"`
	Warning       string        `json:"warning,omitempty"`
}

type writerPolicy struct {
	FallbackMode string `json:"fallback_mode"`
}

type writeMap struct {
	Schema              string          `json:"schema"`
	ImagePath           string          `json:"image_path"`
	DiskBytes           uint64          `json:"disk_bytes"`
	TargetBytes         uint64          `json:"target_bytes"`
	ImageLogicalBytes   uint64          `json:"image_logical_bytes"`
	ImageAllocatedBytes uint64          `json:"image_allocated_bytes"`
	ExtentsStatus       string          `json:"extents_status"`
	RawExtents          []byteRange     `json:"raw_extents"`
	CuratedRanges       []taggedRange   `json:"curated_ranges"`
	WriterPolicy        writerPolicy    `json:"writer_policy"`
	ExtentProbe         extentsOutput   `json:"extent_probe"`
	PartitionTable      json.RawMessage `json:"partition_table"`
	Warning             string          `json:"warning,omitempty"`
}

type sfdiskJSON struct {
	PartitionTable struct {
		SectorSize uint64 `json:"sectorsize"`
		Partitions []struct {
			Node     string `json:"node"`
			Start    uint64 `json:"start"`
			Size     uint64 `json:"size"`
			Type     string `json:"type"`
			Bootable bool   `json:"bootable"`
		} `json:"partitions"`
	} `json:"partitiontable"`
}

func main() {
	image := flag.String("image", "", "Path to sparse image")
	asJSON := flag.Bool("json", false, "Emit JSON output")
	forceFallback := flag.Bool("force-fallback", false, "Force fallback_full mode (testing)")
	partitionJSONPath := flag.String("partition-json", "", "Path to sfdisk -J output")
	emitWriteMap := flag.Bool("emit-write-map", false, "Emit full write-map v2 JSON")
	imagePath := flag.String("image-path", "", "Image path to store in write-map")
	diskBytes := flag.Uint64("disk-bytes", 0, "Source disk bytes")
	targetBytes := flag.Uint64("target-bytes", 0, "Target bytes used for image creation")
	imageAllocatedBytes := flag.Uint64("image-allocated-bytes", 0, "Allocated image bytes")
	flag.Parse()

	if *image == "" {
		fail("--image is required")
	}
	if !*asJSON {
		fail("only --json output is supported")
	}

	probe := buildProbe(*image, *forceFallback)
	if *partitionJSONPath != "" {
		curated, err := buildCuratedRanges(*partitionJSONPath, probe.LogicalBytes)
		if err != nil {
			probe.Warning = joinWarnings(probe.Warning, fmt.Sprintf("curated ranges unavailable: %v", err))
		} else {
			probe.CuratedRanges = curated
		}
	}

	if !*emitWriteMap {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(probe); err != nil {
			fail(err.Error())
		}
		return
	}

	if *imagePath == "" {
		fail("--image-path is required with --emit-write-map")
	}
	if *diskBytes == 0 {
		fail("--disk-bytes is required with --emit-write-map")
	}
	if *targetBytes == 0 {
		fail("--target-bytes is required with --emit-write-map")
	}
	if *imageAllocatedBytes == 0 {
		fail("--image-allocated-bytes is required with --emit-write-map")
	}
	if *partitionJSONPath == "" {
		fail("--partition-json is required with --emit-write-map")
	}

	partitionRaw, err := os.ReadFile(*partitionJSONPath)
	if err != nil {
		fail(fmt.Sprintf("failed to read partition json: %v", err))
	}

	m := writeMap{
		Schema:              "ventoy-dev-image-write-map/v2",
		ImagePath:           *imagePath,
		DiskBytes:           *diskBytes,
		TargetBytes:         *targetBytes,
		ImageLogicalBytes:   probe.LogicalBytes,
		ImageAllocatedBytes: *imageAllocatedBytes,
		ExtentsStatus:       probe.ExtentsStatus,
		RawExtents:          probe.RawExtents,
		CuratedRanges:       probe.CuratedRanges,
		WriterPolicy:        probe.WriterPolicy,
		ExtentProbe:         probe,
		PartitionTable:      json.RawMessage(partitionRaw),
	}
	if probe.Warning != "" {
		m.Warning = probe.Warning
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		fail(err.Error())
	}
}

func buildProbe(imagePath string, forceFallback bool) extentsOutput {
	logical, err := logicalFileSize(imagePath)
	if err != nil {
		fail(err.Error())
	}

	out := extentsOutput{
		Status:        "ok",
		ExtentsStatus: "ok",
		LogicalBytes:  logical,
		Source:        "seek_hole_data",
		WriterPolicy: writerPolicy{
			FallbackMode: "full_write_warn",
		},
	}

	if forceFallback {
		return fullFallback(out, "forced fallback requested")
	}

	if logical == 0 {
		return fullFallback(out, "logical size is zero")
	}

	ranges, err := sparseExtents(imagePath, logical)
	if err != nil {
		return fullFallback(out, fmt.Sprintf("extent scan failed (%s): %v", runtime.GOOS, err))
	}
	if len(ranges) == 0 {
		return fullFallback(out, "extent scan returned no data ranges")
	}

	out.Extents = ranges
	out.RawExtents = ranges
	return out
}

func fullFallback(out extentsOutput, reason string) extentsOutput {
	out.Status = "fallback_full"
	out.ExtentsStatus = "fallback_full"
	out.Source = "fallback_full"
	out.Warning = reason
	out.Extents = []byteRange{{OffsetBytes: 0, LengthBytes: out.LogicalBytes}}
	out.RawExtents = out.Extents
	return out
}

func logicalFileSize(path string) (uint64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("failed to stat image: %w", err)
	}
	if st.Size() < 0 {
		return 0, errors.New("negative file size")
	}
	return uint64(st.Size()), nil
}

func sparseExtents(path string, logical uint64) ([]byteRange, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fd := int(f.Fd())
	var ranges []byteRange
	var off int64
	end := int64(logical)

	for off < end {
		data, err := syscall.Seek(fd, off, seekData)
		if err != nil {
			if errors.Is(err, syscall.ENXIO) {
				break
			}
			return nil, err
		}
		hole, err := syscall.Seek(fd, data, seekHole)
		if err != nil {
			if errors.Is(err, syscall.ENXIO) {
				hole = end
			} else {
				return nil, err
			}
		}
		if hole > end {
			hole = end
		}
		if hole > data {
			ranges = append(ranges, byteRange{
				OffsetBytes: uint64(data),
				LengthBytes: uint64(hole - data),
			})
		}
		off = hole
	}

	return ranges, nil
}

func buildCuratedRanges(partitionJSONPath string, logical uint64) ([]taggedRange, error) {
	raw, err := os.ReadFile(partitionJSONPath)
	if err != nil {
		return nil, err
	}

	var table sfdiskJSON
	if err := json.Unmarshal(raw, &table); err != nil {
		return nil, fmt.Errorf("failed to parse partition json: %w", err)
	}
	if len(table.PartitionTable.Partitions) == 0 {
		return nil, errors.New("no partitions in partition table")
	}

	sectorSize := table.PartitionTable.SectorSize
	if sectorSize == 0 {
		sectorSize = 512
	}

	parts := table.PartitionTable.Partitions
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].Start < parts[j].Start
	})

	p1 := parts[0]
	p2 := parts[len(parts)-1]
	for _, p := range parts {
		if p.Type == "ef" || p.Type == "EF" {
			p2 = p
		}
	}

	var out []taggedRange
	add := func(tag string, off, length uint64) {
		if length == 0 {
			return
		}
		if off >= logical {
			return
		}
		if off+length > logical {
			length = logical - off
		}
		if length == 0 {
			return
		}
		out = append(out, taggedRange{
			Tag: tag,
			byteRange: byteRange{
				OffsetBytes: off,
				LengthBytes: length,
			},
		})
	}

	// BIOS boot code in protective/legacy MBR area.
	add("mbr_boot", 0, 446)

	// Ventoy core area in the post-MBR gap (starts at LBA34 and ends before p1 start).
	postGapStart := uint64(34) * sectorSize
	p1Start := p1.Start * sectorSize
	if p1Start > postGapStart {
		add("post_mbr_gap", postGapStart, p1Start-postGapStart)
	}

	// Ventoy EFI payload partition.
	add("part2_vtoyefi", p2.Start*sectorSize, p2.Size*sectorSize)

	// Hint for future selective write/expansion of data partition (only leading slice).
	const hintCap = 16 * 1024 * 1024
	p1Len := p1.Size * sectorSize
	if p1Len > hintCap {
		p1Len = hintCap
	}
	add("part1_hint", p1.Start*sectorSize, p1Len)

	if len(out) == 0 {
		return nil, errors.New("no curated ranges derived")
	}
	return out, nil
}

func joinWarnings(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "; " + b
}

func fail(msg string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	os.Exit(1)
}
