package ventoy

const (
	SectorSize = uint64(512)
	MapSchema  = "ventoy-toolkit-mbr-transplant-map"
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
