package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type sfdiskJSON struct {
	PartitionTable struct {
		SectorSize uint64 `json:"sectorsize"`
		Partitions []struct {
			Start    uint64 `json:"start"`
			Size     uint64 `json:"size"`
			Type     string `json:"type"`
			Bootable bool   `json:"bootable"`
		} `json:"partitions"`
	} `json:"partitiontable"`
}

func BuildWriteMap(image, imagePath, partitionJSON string) (WriteMap, error) {
	logicalBytes, err := fileSize(image)
	if err != nil {
		return WriteMap{}, err
	}
	table, err := readPartitionTable(partitionJSON)
	if err != nil {
		return WriteMap{}, err
	}
	zones, err := derivedZones(table)
	if err != nil {
		return WriteMap{}, err
	}
	if imagePath == "" {
		imagePath = image
	}
	return WriteMap{
		Schema:            mapSchema,
		ImagePath:         imagePath,
		ImageLogicalBytes: logicalBytes,
		PartitionTable:    table,
		DerivedZones:      zones,
	}, nil
}

func ReadWriteMap(path string) (WriteMap, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return WriteMap{}, fmt.Errorf("failed to read map: %w", err)
	}
	var m WriteMap
	if err := json.Unmarshal(raw, &m); err != nil {
		return WriteMap{}, fmt.Errorf("failed to parse map: %w", err)
	}
	return m, nil
}

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

func readPartitionTable(path string) (PartitionTable, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return PartitionTable{}, err
	}

	var sfdisk sfdiskJSON
	if err := json.Unmarshal(raw, &sfdisk); err != nil {
		return PartitionTable{}, fmt.Errorf("failed to parse sfdisk json: %w", err)
	}

	table := PartitionTable{SectorSize: sfdisk.PartitionTable.SectorSize}
	if table.SectorSize == 0 {
		table.SectorSize = sectorSize
	}
	for i, p := range sfdisk.PartitionTable.Partitions {
		if p.Size == 0 {
			continue
		}
		table.Partitions = append(table.Partitions, Partition{
			Index:       i + 1,
			TypeHex:     normalizeType(p.Type),
			Bootable:    p.Bootable,
			StartSector: p.Start,
			SizeSectors: p.Size,
			OffsetBytes: p.Start * table.SectorSize,
			LengthBytes: p.Size * table.SectorSize,
		})
	}
	if len(table.Partitions) < 2 {
		return PartitionTable{}, errors.New("expected at least two partitions")
	}
	return table, nil
}

func derivedZones(table PartitionTable) (DerivedZones, error) {
	if len(table.Partitions) < 2 {
		return DerivedZones{}, errors.New("expected two partitions")
	}
	p1, p2 := table.Partitions[0], table.Partitions[1]
	return DerivedZones{
		PreP1:     newZone(0, p1.OffsetBytes, table.SectorSize),
		P1Data:    newZone(p1.OffsetBytes, p1.LengthBytes, table.SectorSize),
		P2VtoyEFI: newZone(p2.OffsetBytes, p2.LengthBytes, table.SectorSize),
	}, nil
}

func newZone(offset, length, sectorSize uint64) Zone {
	zone := Zone{OffsetBytes: offset, LengthBytes: length, StartSector: offset / sectorSize}
	if length > 0 {
		zone.EndSector = (offset + length - 1) / sectorSize
	}
	return zone
}

func normalizeType(value string) string {
	value = strings.ToLower(strings.TrimPrefix(value, "0x"))
	if len(value) == 1 {
		value = "0" + value
	}
	return "0x" + value
}
