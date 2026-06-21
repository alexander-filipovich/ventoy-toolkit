package writer

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/alexanderfilipovich/ventoy-toolkit/cmd/ventoyctl/ventoy"
)

func verifyTargetMBR(rawDisk string, expected []byte) error {
	f, err := os.Open(rawDisk)
	if err != nil {
		return fmt.Errorf("failed to open target MBR: %w", err)
	}
	defer f.Close()

	sector := make([]byte, ventoy.SectorSize)
	if _, err := io.ReadFull(f, sector); err != nil {
		return fmt.Errorf("failed to read target MBR: %w", err)
	}
	return verifyMBRSector(sector, expected)
}

func verifyMBRSector(sector, expected []byte) error {
	if len(sector) < int(ventoy.SectorSize) {
		return fmt.Errorf("target MBR is too short")
	}
	if len(expected) < int(ventoy.SectorSize) {
		return fmt.Errorf("expected MBR is too short")
	}
	if sector[510] != 0x55 || sector[511] != 0xaa {
		return fmt.Errorf("target MBR signature is invalid")
	}
	if !bytes.Equal(sector[446:478], expected[446:478]) {
		return fmt.Errorf("target MBR partition entries do not match patched image MBR")
	}
	return nil
}
