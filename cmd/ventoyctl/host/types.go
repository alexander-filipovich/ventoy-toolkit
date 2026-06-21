package host

type diskListResult struct {
	WholeDisks []string `json:"WholeDisks"`
}

type diskInfo struct {
	DeviceNode                       string `json:"DeviceNode"`
	DeviceIdentifier                 string `json:"DeviceIdentifier"`
	TotalSize                        uint64 `json:"TotalSize"`
	MediaName                        string `json:"MediaName"`
	VolumeName                       string `json:"VolumeName"`
	IORegistryEntryName              string `json:"IORegistryEntryName"`
	WholeDisk                        bool   `json:"WholeDisk"`
	Internal                         bool   `json:"Internal"`
	RemovableMediaOrExternalDevice   bool   `json:"RemovableMediaOrExternalDevice"`
	PartitionMapPartitionOffsetBytes uint64 `json:"PartitionMapPartitionOffset"`
}

type Disk struct {
	ID        string
	Path      string
	SizeBytes uint64
	SizeHuman string
	Name      string
}
