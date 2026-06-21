package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

func BuildWritePlan(m WriteMap, targetBytes uint64) (WritePlan, error) {
	if err := validateMap(m, targetBytes); err != nil {
		return WritePlan{}, err
	}
	p1, p2 := m.PartitionTable.Partitions[0], m.PartitionTable.Partitions[1]
	targetSectors := targetBytes / sectorSize
	if targetSectors <= p1.StartSector+p2.SizeSectors {
		return WritePlan{}, fmt.Errorf("target disk is too small")
	}

	newP2Start := targetSectors - p2.SizeSectors
	newP1Size := newP2Start - p1.StartSector
	if newP2Start > uint64(^uint32(0)) || newP1Size > uint64(^uint32(0)) {
		return WritePlan{}, fmt.Errorf("expanded MBR layout exceeds 32-bit sector fields")
	}

	ranges := []CopyRange{
		{ID: "patch-mbr", Source: "__PATCHED_MBR__", SrcOff: 0, DstOff: 0, Length: sectorSize},
		{ID: "pre-p1-tail", Source: "__IMAGE__", SrcOff: sectorSize, DstOff: sectorSize, Length: m.DerivedZones.PreP1.LengthBytes - sectorSize},
		{ID: "p2-vtoyefi", Source: "__IMAGE__", SrcOff: p2.OffsetBytes, DstOff: newP2Start * sectorSize, Length: p2.LengthBytes},
	}

	writeBytes := uint64(0)
	for _, r := range ranges {
		writeBytes += r.Length
	}
	return WritePlan{Map: m, TargetBytes: targetBytes, P1: p1, P2: p2, NewP1Size: newP1Size, NewP2Start: newP2Start, Ranges: ranges, WriteBytes: writeBytes}, nil
}

func PatchedMBR(imagePath string, plan WritePlan) ([]byte, error) {
	f, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer f.Close()

	mbr := make([]byte, sectorSize)
	if _, err := io.ReadFull(f, mbr); err != nil {
		return nil, fmt.Errorf("failed to read MBR: %w", err)
	}
	if mbr[510] != 0x55 || mbr[511] != 0xaa {
		return nil, fmt.Errorf("image MBR signature is invalid")
	}
	writeMBREntry(mbr, 0, uint32(plan.P1.StartSector), uint32(plan.NewP1Size))
	writeMBREntry(mbr, 1, uint32(plan.NewP2Start), uint32(plan.P2.SizeSectors))
	return mbr, nil
}

func validateMap(m WriteMap, targetBytes uint64) error {
	if m.Schema != mapSchema {
		return fmt.Errorf("unsupported write-map schema")
	}
	if m.ImagePath == "" || m.ImageLogicalBytes == 0 {
		return fmt.Errorf("write-map image fields are empty")
	}
	if m.PartitionTable.SectorSize != sectorSize {
		return fmt.Errorf("only 512-byte sector maps are supported")
	}
	if len(m.PartitionTable.Partitions) < 2 {
		return fmt.Errorf("write-map must contain two partitions")
	}
	if targetBytes%sectorSize != 0 {
		return fmt.Errorf("target size is not sector-aligned")
	}

	p1, p2 := m.PartitionTable.Partitions[0], m.PartitionTable.Partitions[1]
	if p1.TypeHex != "0x07" || p2.TypeHex != "0xef" {
		return fmt.Errorf("unexpected partition types: p1=%s p2=%s", p1.TypeHex, p2.TypeHex)
	}
	if p1.OffsetBytes != m.DerivedZones.PreP1.LengthBytes || p2.OffsetBytes != m.DerivedZones.P2VtoyEFI.OffsetBytes {
		return fmt.Errorf("derived zones do not match partition table")
	}
	if m.DerivedZones.PreP1.LengthBytes < sectorSize {
		return fmt.Errorf("pre_p1 zone is smaller than one sector")
	}
	if m.DerivedZones.P1Data.OffsetBytes != p1.OffsetBytes || m.DerivedZones.P1Data.LengthBytes != p1.LengthBytes {
		return fmt.Errorf("p1_data zone does not match partition 1")
	}
	if m.DerivedZones.P2VtoyEFI.LengthBytes != p2.LengthBytes {
		return fmt.Errorf("p2_vtoyefi zone does not match partition 2")
	}
	return nil
}

func writeMBREntry(mbr []byte, index int, start, size uint32) {
	off := 446 + index*16
	mbr[off+1], mbr[off+2], mbr[off+3] = 0xfe, 0xff, 0xff
	mbr[off+5], mbr[off+6], mbr[off+7] = 0xfe, 0xff, 0xff
	binary.LittleEndian.PutUint32(mbr[off+8:off+12], start)
	binary.LittleEndian.PutUint32(mbr[off+12:off+16], size)
}

func PrintPlan(plan WritePlan, imagePath, rawDisk string) {
	fmt.Printf("image=%s\n", imagePath)
	fmt.Printf("target_size=%s (%d B)\n", humanBytes(plan.TargetBytes), plan.TargetBytes)
	fmt.Printf("p1_start_sector=%d\n", plan.P1.StartSector)
	fmt.Printf("p1_new_size=%s (%d sectors)\n", humanBytes(plan.NewP1Size*sectorSize), plan.NewP1Size)
	fmt.Printf("p2_new_start_sector=%d\n", plan.NewP2Start)
	fmt.Printf("p2_size=%s (%d sectors)\n", humanBytes(plan.P2.SizeSectors*sectorSize), plan.P2.SizeSectors)
	fmt.Printf("write_bytes_total=%s (%d B)\n", humanBytes(plan.WriteBytes), plan.WriteBytes)
	for _, r := range plan.Ranges {
		source := r.Source
		if source == "__IMAGE__" {
			source = imagePath
		}
		bs := bestBlockSize(r.SrcOff, r.DstOff, r.Length)
		fmt.Printf("range=%s dd if=%s of=%s bs=%d skip=%d seek=%d count=%d conv=notrunc\n",
			r.ID, source, rawDisk, bs, r.SrcOff/bs, r.DstOff/bs, r.Length/bs)
	}
}

func bestBlockSize(srcOff, dstOff, length uint64) uint64 {
	for _, candidate := range []uint64{4 * 1024 * 1024, 1024 * 1024, 64 * 1024, sectorSize} {
		if srcOff%candidate == 0 && dstOff%candidate == 0 && length%candidate == 0 {
			return candidate
		}
	}
	return sectorSize
}

func humanBytes(bytes uint64) string {
	switch {
	case bytes >= 1<<40:
		return fmt.Sprintf("%.2f TiB", float64(bytes)/(1<<40))
	case bytes >= 1<<30:
		return fmt.Sprintf("%.2f GiB", float64(bytes)/(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.2f MiB", float64(bytes)/(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(bytes)/(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
