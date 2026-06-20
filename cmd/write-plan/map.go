package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func readMap(path string) writeMap {
	raw, err := os.ReadFile(path)
	if err != nil {
		fail(fmt.Sprintf("failed to read map: %v", err))
	}
	var m writeMap
	if err := json.Unmarshal(raw, &m); err != nil {
		fail(fmt.Sprintf("failed to parse map: %v", err))
	}
	return m
}

func buildLayout(m writeMap, targetBytes uint64) layout {
	validateMapBasics(m, targetBytes)

	p1 := m.PartitionTable.Partitions[0]
	p2 := m.PartitionTable.Partitions[1]
	validatePartitions(m, p1, p2)

	targetSectors := targetBytes / sectorSize
	if targetSectors <= p1.StartSector+p2.SizeSectors {
		fail("target disk is too small")
	}

	newP2Start := targetSectors - p2.SizeSectors
	newP1Size := newP2Start - p1.StartSector
	if newP2Start > uint64(^uint32(0)) || newP1Size > uint64(^uint32(0)) {
		fail("expanded MBR layout exceeds 32-bit sector fields")
	}

	return layout{p1: p1, p2: p2, targetBytes: targetBytes, newP1Size: newP1Size, newP2Start: newP2Start}
}

func validateMapBasics(m writeMap, targetBytes uint64) {
	if m.Schema != "ventoy-dev-image-write-map" {
		fail("unsupported write-map schema")
	}
	if m.ImagePath == "" || m.ImageLogicalBytes == 0 {
		fail("write-map image fields are empty")
	}
	if m.PartitionTable.SectorSize != sectorSize {
		fail("only 512-byte sector maps are supported")
	}
	if len(m.PartitionTable.Partitions) < 2 {
		fail("write-map must contain two partitions")
	}
	if targetBytes%sectorSize != 0 {
		fail("target size is not sector-aligned")
	}
}

func validatePartitions(m writeMap, p1, p2 partition) {
	if p1.TypeHex != "0x07" || p2.TypeHex != "0xef" {
		fail(fmt.Sprintf("unexpected partition types: p1=%s p2=%s", p1.TypeHex, p2.TypeHex))
	}
	if p1.OffsetBytes != m.DerivedZones.PreP1.LengthBytes || p2.OffsetBytes != m.DerivedZones.P2VtoyEFI.OffsetBytes {
		fail("derived zones do not match partition table")
	}
	if m.DerivedZones.PreP1.LengthBytes < sectorSize {
		fail("pre_p1 zone is smaller than one sector")
	}
	if m.DerivedZones.P1Data.OffsetBytes != p1.OffsetBytes || m.DerivedZones.P1Data.LengthBytes != p1.LengthBytes {
		fail("p1_data zone does not match partition 1")
	}
	if m.DerivedZones.P2VtoyEFI.LengthBytes != p2.LengthBytes {
		fail("p2_vtoyefi zone does not match partition 2")
	}
}
