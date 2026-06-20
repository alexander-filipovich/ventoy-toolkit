package main

const sectorSize = 512

type partition struct {
	Index       int    `json:"index"`
	TypeHex     string `json:"type_hex"`
	Bootable    bool   `json:"bootable"`
	StartSector uint64 `json:"start_sector"`
	SizeSectors uint64 `json:"size_sectors"`
	OffsetBytes uint64 `json:"offset_bytes"`
	LengthBytes uint64 `json:"length_bytes"`
}

type partitionTable struct {
	SectorSize uint64      `json:"sector_size"`
	Partitions []partition `json:"partitions"`
}

type zone struct {
	OffsetBytes uint64 `json:"offset_bytes"`
	LengthBytes uint64 `json:"length_bytes"`
}

type derivedZones struct {
	PreP1     zone `json:"pre_p1"`
	P1Data    zone `json:"p1_data"`
	P2VtoyEFI zone `json:"p2_vtoyefi"`
}

type writeMap struct {
	Schema            string         `json:"schema"`
	ImagePath         string         `json:"image_path"`
	ImageLogicalBytes uint64         `json:"image_logical_bytes"`
	PartitionTable    partitionTable `json:"partition_table"`
	DerivedZones      derivedZones   `json:"derived_zones"`
}

type layout struct {
	p1          partition
	p2          partition
	targetBytes uint64
	newP1Size   uint64
	newP2Start  uint64
}

type copyRange struct {
	id     string
	src    string
	srcOff uint64
	dstOff uint64
	length uint64
}
