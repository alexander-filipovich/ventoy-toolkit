package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type writeOptions struct {
	MapPath string
	Disk    string
	Confirm string
	DryRun  bool
}

func Write(opts writeOptions) error {
	m, err := ReadWriteMap(opts.MapPath)
	if err != nil {
		return err
	}

	diskID, err := normalizeDiskID(opts.Disk)
	if err != nil {
		return err
	}

	var disk Disk
	if opts.DryRun {
		disk = Disk{ID: diskID, Path: "/dev/" + diskID, SizeBytes: m.ImageLogicalBytes, SizeHuman: humanBytes(m.ImageLogicalBytes)}
		if realDisk, err := ValidateTargetDisk(diskID); err == nil {
			disk = realDisk
		} else {
			fmt.Fprintf(os.Stderr, "[ventoyctl] dry-run: disk info unavailable for /dev/%s, using image size for plan\n", diskID)
		}
	} else {
		if os.Geteuid() != 0 {
			return fmt.Errorf("run as root (use sudo) for real writes")
		}
		disk, err = ValidateTargetDisk(diskID)
		if err != nil {
			return err
		}
		if opts.Confirm == "" {
			fmt.Fprintf(os.Stderr, "Type '%s' to confirm writing Ventoy layout to /dev/r%s and formatting /dev/%ss1:\n", disk.ID, disk.ID, disk.ID)
			if _, err := fmt.Fscan(os.Stdin, &opts.Confirm); err != nil {
				return fmt.Errorf("failed to read confirmation: %w", err)
			}
		}
		if opts.Confirm != disk.ID {
			return fmt.Errorf("confirmation mismatch (expected '%s')", disk.ID)
		}
	}

	imagePath := resolveImagePath(opts.MapPath, m.ImagePath)
	imageBytes, err := fileSize(imagePath)
	if err != nil {
		return err
	}
	if imageBytes != m.ImageLogicalBytes {
		return fmt.Errorf("image size mismatch (map=%d, file=%d)", m.ImageLogicalBytes, imageBytes)
	}

	plan, err := BuildWritePlan(m, disk.SizeBytes)
	if err != nil {
		return err
	}

	rawDisk := "/dev/r" + disk.ID
	if opts.DryRun {
		PrintPlan(plan, imagePath, rawDisk)
		fmt.Printf("dry-run: diskutil unmountDisk force %s\n", disk.Path)
		fmt.Printf("dry-run: diskutil eraseVolume ExFAT Ventoy %ss1\n", disk.Path)
		fmt.Println("dry-run: sync")
		return nil
	}

	tmpDir, err := os.MkdirTemp("", "ventoyctl-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	patchPath := filepath.Join(tmpDir, "patched-mbr.bin")
	patch, err := PatchedMBR(imagePath, plan)
	if err != nil {
		return err
	}
	if err := os.WriteFile(patchPath, patch, 0o600); err != nil {
		return err
	}

	_ = runInherit("diskutil", "unmountDisk", "force", disk.Path)
	for _, r := range plan.Ranges {
		source := r.Source
		if source == "__IMAGE__" {
			source = imagePath
		}
		if source == "__PATCHED_MBR__" {
			source = patchPath
		}
		if err := copyRange(source, rawDisk, r); err != nil {
			return err
		}
	}
	if err := runInherit("sync"); err != nil {
		return err
	}

	_ = runInherit("diskutil", "list", disk.Path)
	if offset, ok := partitionOffset(disk.Path + "s1"); ok && offset != plan.P1.StartSector*sectorSize {
		return fmt.Errorf("%ss1 offset mismatch after write", disk.Path)
	}
	_ = runInherit("diskutil", "unmountDisk", "force", disk.Path)
	if err := runInherit("diskutil", "eraseVolume", "ExFAT", "Ventoy", disk.Path+"s1"); err != nil {
		return err
	}
	return runInherit("sync")
}

func copyRange(source, rawDisk string, r CopyRange) error {
	if r.SrcOff%sectorSize != 0 || r.DstOff%sectorSize != 0 || r.Length%sectorSize != 0 {
		return fmt.Errorf("%s range is not sector-aligned", r.ID)
	}
	bs := bestBlockSize(r.SrcOff, r.DstOff, r.Length)
	fmt.Fprintf(os.Stderr, "[ventoyctl] %s: %s -> %s length=%s\n", r.ID, source, rawDisk, humanBytes(r.Length))
	return runInherit("dd",
		"if="+source,
		"of="+rawDisk,
		fmt.Sprintf("bs=%d", bs),
		fmt.Sprintf("skip=%d", r.SrcOff/bs),
		fmt.Sprintf("seek=%d", r.DstOff/bs),
		fmt.Sprintf("count=%d", r.Length/bs),
		"conv=notrunc",
	)
}

func resolveImagePath(mapPath, imagePath string) string {
	if filepath.IsAbs(imagePath) {
		return imagePath
	}
	if _, err := os.Stat(imagePath); err == nil {
		return imagePath
	}
	return filepath.Join(filepath.Dir(mapPath), imagePath)
}
