package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const defaultSectorSize = 512

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

func readPartitionTable(path string) (partitionTable, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return partitionTable{}, err
	}

	var sfdisk sfdiskJSON
	if err := json.Unmarshal(raw, &sfdisk); err != nil {
		return partitionTable{}, fmt.Errorf("failed to parse sfdisk json: %w", err)
	}

	sectorSize := sfdisk.PartitionTable.SectorSize
	if sectorSize == 0 {
		sectorSize = defaultSectorSize
	}

	table := partitionTable{SectorSize: sectorSize}
	for i, p := range sfdisk.PartitionTable.Partitions {
		if p.Size == 0 {
			continue
		}
		table.Partitions = append(table.Partitions, partition{
			Index:       i + 1,
			TypeHex:     normalizeType(p.Type),
			Bootable:    p.Bootable,
			StartSector: p.Start,
			SizeSectors: p.Size,
			OffsetBytes: p.Start * sectorSize,
			LengthBytes: p.Size * sectorSize,
		})
	}
	if len(table.Partitions) < 2 {
		return partitionTable{}, errors.New("expected at least two partitions")
	}
	return table, nil
}

func normalizeType(t string) string {
	t = strings.ToLower(t)
	t = strings.TrimPrefix(t, "0x")
	if len(t) == 1 {
		t = "0" + t
	}
	return "0x" + t
}

func buildDerivedZones(table partitionTable) (derivedZones, error) {
	p1, p2, err := firstTwoPartitions(table)
	if err != nil {
		return derivedZones{}, err
	}
	return derivedZones{
		PreP1:     newZone(0, p1.OffsetBytes, table.SectorSize),
		P1Data:    newZone(p1.OffsetBytes, p1.LengthBytes, table.SectorSize),
		P2VtoyEFI: newZone(p2.OffsetBytes, p2.LengthBytes, table.SectorSize),
	}, nil
}

func firstTwoPartitions(table partitionTable) (partition, partition, error) {
	if len(table.Partitions) < 2 {
		return partition{}, partition{}, errors.New("expected two partitions")
	}
	return table.Partitions[0], table.Partitions[1], nil
}

func newZone(offset, length, sectorSize uint64) zone {
	z := zone{OffsetBytes: offset, LengthBytes: length, StartSector: offset / sectorSize}
	if length > 0 {
		z.EndSector = (offset + length - 1) / sectorSize
	}
	return z
}
