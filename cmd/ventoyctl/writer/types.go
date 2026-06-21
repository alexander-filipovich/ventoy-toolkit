package writer

import (
	"github.com/alexanderfilipovich/ventoy-toolkit/cmd/ventoyctl/ventoy"
)

type Options struct {
	MapPath string
	Disk    string
	Confirm string
	DryRun  bool
}

type WritePlan struct {
	TargetBytes uint64
	P1          ventoy.Partition
	P2          ventoy.Partition
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
