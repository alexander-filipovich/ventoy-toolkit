package main

type byteRange struct {
	OffsetBytes uint64 `json:"offset_bytes"`
	LengthBytes uint64 `json:"length_bytes"`
	StartSector uint64 `json:"start_sector"`
	EndSector   uint64 `json:"end_sector"`
}

type holeInfo struct {
	Status  string    `json:"status"`
	Range   byteRange `json:"range,omitempty"`
	Warning string    `json:"warning,omitempty"`
}

type writeMap struct {
	Schema            string      `json:"schema"`
	ImagePath         string      `json:"image_path"`
	ImageLogicalBytes uint64      `json:"image_logical_bytes"`
	ExtentsStatus     string      `json:"extents_status"`
	RawExtents        []byteRange `json:"raw_extents"`
	ExpansionHole     holeInfo    `json:"expansion_hole"`
	Warning           string      `json:"warning,omitempty"`
}
