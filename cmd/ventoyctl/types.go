package main

const (
	sectorSize = uint64(512)
	mapSchema  = "ventoy-dev-image-write-map"
)

type WriteMap struct {
	Schema            string         `json:"schema"`
	ImagePath         string         `json:"image_path"`
	ImageLogicalBytes uint64         `json:"image_logical_bytes"`
	PartitionTable    PartitionTable `json:"partition_table"`
	DerivedZones      DerivedZones   `json:"derived_zones"`
}

type PartitionTable struct {
	SectorSize uint64      `json:"sector_size"`
	Partitions []Partition `json:"partitions"`
}

type Partition struct {
	Index       int    `json:"index"`
	TypeHex     string `json:"type_hex"`
	Bootable    bool   `json:"bootable"`
	StartSector uint64 `json:"start_sector"`
	SizeSectors uint64 `json:"size_sectors"`
	OffsetBytes uint64 `json:"offset_bytes"`
	LengthBytes uint64 `json:"length_bytes"`
}

type DerivedZones struct {
	PreP1     Zone `json:"pre_p1"`
	P1Data    Zone `json:"p1_data"`
	P2VtoyEFI Zone `json:"p2_vtoyefi"`
}

type Zone struct {
	OffsetBytes uint64 `json:"offset_bytes"`
	LengthBytes uint64 `json:"length_bytes"`
	StartSector uint64 `json:"start_sector,omitempty"`
	EndSector   uint64 `json:"end_sector,omitempty"`
}

type WritePlan struct {
	Map         WriteMap
	TargetBytes uint64
	P1          Partition
	P2          Partition
	NewP1Size   uint64
	NewP2Start  uint64
	Ranges      []CopyRange
	WriteBytes  uint64
}

type CopyRange struct {
	ID     string
	Source string
	SrcOff uint64
	DstOff uint64
	Length uint64
}

type Disk struct {
	ID        string
	Path      string
	SizeBytes uint64
	SizeHuman string
	Name      string
}
