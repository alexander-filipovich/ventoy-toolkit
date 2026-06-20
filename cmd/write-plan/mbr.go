package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

func patchedMBR(imagePath string, l layout) []byte {
	f, err := os.Open(imagePath)
	if err != nil {
		fail(fmt.Sprintf("failed to open image: %v", err))
	}
	defer f.Close()

	mbr := make([]byte, sectorSize)
	if _, err := io.ReadFull(f, mbr); err != nil {
		fail(fmt.Sprintf("failed to read MBR: %v", err))
	}
	if mbr[510] != 0x55 || mbr[511] != 0xaa {
		fail("image MBR signature is invalid")
	}
	writeMBREntry(mbr, 0, uint32(l.p1.StartSector), uint32(l.newP1Size))
	writeMBREntry(mbr, 1, uint32(l.newP2Start), uint32(l.p2.SizeSectors))
	return mbr
}

func writeMBREntry(mbr []byte, index int, start, size uint32) {
	off := 446 + index*16
	mbr[off+1], mbr[off+2], mbr[off+3] = 0xfe, 0xff, 0xff
	mbr[off+5], mbr[off+6], mbr[off+7] = 0xfe, 0xff, 0xff
	binary.LittleEndian.PutUint32(mbr[off+8:off+12], start)
	binary.LittleEndian.PutUint32(mbr[off+12:off+16], size)
}
