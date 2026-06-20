package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func writePlan(m writeMap, l layout, envPath, rangesPath string) {
	patchPath := filepath.Join(filepath.Dir(rangesPath), "patched-mbr.bin")
	if err := os.WriteFile(patchPath, patchedMBR(m.ImagePath, l), 0o600); err != nil {
		fail(err.Error())
	}

	ranges := []copyRange{
		{id: "patch-mbr", src: patchPath, srcOff: 0, dstOff: 0, length: sectorSize},
		{id: "pre-p1-tail", src: "__IMAGE__", srcOff: sectorSize, dstOff: sectorSize, length: m.DerivedZones.PreP1.LengthBytes - sectorSize},
		{id: "p2-vtoyefi", src: "__IMAGE__", srcOff: l.p2.OffsetBytes, dstOff: l.newP2Start * sectorSize, length: l.p2.LengthBytes},
	}

	writeBytes := writeRanges(rangesPath, ranges)
	writeMetadata(envPath, m, l, writeBytes)
}

func writeRanges(path string, ranges []copyRange) uint64 {
	f := mustCreate(path)
	defer mustClose(f)

	writeBytes := uint64(0)
	for _, r := range ranges {
		if r.length == 0 {
			continue
		}
		writeBytes += r.length
		if _, err := fmt.Fprintf(f, "%s\t%s\t%d\t%d\t%d\n", r.id, r.src, r.srcOff, r.dstOff, r.length); err != nil {
			fail(err.Error())
		}
	}
	return writeBytes
}

func writeMetadata(path string, m writeMap, l layout, writeBytes uint64) {
	f := mustCreate(path)
	defer mustClose(f)

	writeEnv(f, "IMAGE_PATH", m.ImagePath)
	writeEnv(f, "IMAGE_LOGICAL_BYTES", fmt.Sprintf("%d", m.ImageLogicalBytes))
	writeEnv(f, "TARGET_BYTES", fmt.Sprintf("%d", l.targetBytes))
	writeEnv(f, "SECTOR_SIZE", fmt.Sprintf("%d", sectorSize))
	writeEnv(f, "P1_START_SECTOR", fmt.Sprintf("%d", l.p1.StartSector))
	writeEnv(f, "P1_OLD_SIZE_SECTORS", fmt.Sprintf("%d", l.p1.SizeSectors))
	writeEnv(f, "P1_NEW_SIZE_SECTORS", fmt.Sprintf("%d", l.newP1Size))
	writeEnv(f, "P2_OLD_START_SECTOR", fmt.Sprintf("%d", l.p2.StartSector))
	writeEnv(f, "P2_NEW_START_SECTOR", fmt.Sprintf("%d", l.newP2Start))
	writeEnv(f, "P2_SIZE_SECTORS", fmt.Sprintf("%d", l.p2.SizeSectors))
	writeEnv(f, "WRITE_BYTES", fmt.Sprintf("%d", writeBytes))
}
