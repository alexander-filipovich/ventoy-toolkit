package main

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"golang.org/x/sys/unix"
)

// MBR/LBA coordinates in the raw Ventoy image are expressed in 512-byte sectors.
const mbrSectorSize = 512

func fileSize(path string) (uint64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("failed to stat image: %w", err)
	}
	if st.Size() <= 0 {
		return 0, errors.New("image must not be empty")
	}
	return uint64(st.Size()), nil
}

func scanExtents(path string, logicalBytes uint64, forceFallback bool) ([]byteRange, string, string) {
	if forceFallback {
		return fullRange(logicalBytes, "forced fallback requested")
	}

	ranges, err := platformSparseExtents(path, logicalBytes)
	if err != nil {
		return fullRange(logicalBytes, fmt.Sprintf("extent scan failed: %v", err))
	}
	if len(ranges) == 0 {
		return fullRange(logicalBytes, "extent scan returned no data ranges")
	}
	return ranges, "ok", ""
}

func fullRange(logicalBytes uint64, warning string) ([]byteRange, string, string) {
	return []byteRange{newByteRange(0, logicalBytes)}, "fallback_full", warning
}

func platformSparseExtents(path string, logicalBytes uint64) ([]byteRange, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fd := int(f.Fd())
	end := int64(logicalBytes)
	var ranges []byteRange

	for offset := int64(0); offset < end; {
		data, err := unix.Seek(fd, offset, unix.SEEK_DATA)
		if errors.Is(err, unix.ENXIO) {
			break
		}
		if err != nil {
			return nil, err
		}

		hole, err := unix.Seek(fd, data, unix.SEEK_HOLE)
		if errors.Is(err, unix.ENXIO) || hole > end {
			hole = end
		} else if err != nil {
			return nil, err
		}

		ranges = append(ranges, newByteRange(uint64(data), uint64(hole-data)))
		offset = hole
	}
	return ranges, nil
}

func newByteRange(offset, length uint64) byteRange {
	return byteRange{
		OffsetBytes: offset,
		LengthBytes: length,
		StartSector: offset / mbrSectorSize,
		EndSector:   (offset + length - 1) / mbrSectorSize,
	}
}

func findExpansionHole(logicalBytes uint64, ranges []byteRange) holeInfo {
	holes := holesBetween(logicalBytes, ranges)
	if len(holes) == 0 {
		return holeInfo{Status: "none", Warning: "no holes found"}
	}

	largest := holes[0]
	for _, h := range holes {
		if h.LengthBytes > largest.LengthBytes {
			largest = h
		}
	}

	status := "ok"
	warning := ""
	if largest.LengthBytes <= logicalBytes/2 {
		status = "too_small"
		warning = "largest hole is not greater than half of image"
	}

	return holeInfo{
		Status:  status,
		Range:   largest,
		Warning: warning,
	}
}

func holesBetween(logicalBytes uint64, ranges []byteRange) []byteRange {
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].OffsetBytes < ranges[j].OffsetBytes
	})

	var holes []byteRange
	var cursor uint64
	for _, r := range ranges {
		if r.OffsetBytes > cursor {
			holes = append(holes, newByteRange(cursor, r.OffsetBytes-cursor))
		}
		end := r.OffsetBytes + r.LengthBytes
		if end > cursor {
			cursor = end
		}
	}
	if cursor < logicalBytes {
		holes = append(holes, newByteRange(cursor, logicalBytes-cursor))
	}
	return holes
}
