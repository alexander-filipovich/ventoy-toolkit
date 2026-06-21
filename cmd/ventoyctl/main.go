package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "list-disks":
		err = cmdListDisks(os.Args[2:])
	case "map-image":
		err = cmdMapImage(os.Args[2:])
	case "write":
		err = cmdWrite(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		err = fmt.Errorf("unknown command: %s", os.Args[1])
	}
	if err != nil {
		fail(err.Error())
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage:
  ventoyctl list-disks
  ventoyctl map-image --image PATH --image-path PATH --partition-json PATH
  ventoyctl write --map PATH --disk diskN [--confirm diskN] [--dry-run]`)
}

func cmdListDisks(args []string) error {
	fs := flag.NewFlagSet("list-disks", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	disks, err := ListExternalDisks()
	if err != nil {
		return err
	}
	for _, disk := range disks {
		fmt.Printf("%s\t%s\t%s\n", disk.Path, disk.SizeHuman, disk.Name)
	}
	return nil
}

func cmdMapImage(args []string) error {
	fs := flag.NewFlagSet("map-image", flag.ContinueOnError)
	image := fs.String("image", "", "Path to reference image")
	imagePath := fs.String("image-path", "", "Image path stored in the write map")
	partitionJSON := fs.String("partition-json", "", "Path to sfdisk -J JSON")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if *image == "" || *partitionJSON == "" {
		return fmt.Errorf("--image and --partition-json are required")
	}
	m, err := BuildWriteMap(*image, *imagePath, *partitionJSON)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

func cmdWrite(args []string) error {
	fs := flag.NewFlagSet("write", flag.ContinueOnError)
	mapPath := fs.String("map", "", "Path to write-map JSON")
	disk := fs.String("disk", "", "Target disk, e.g. disk4 or /dev/disk4")
	confirm := fs.String("confirm", "", "Confirmation token, e.g. disk4")
	dryRun := fs.Bool("dry-run", false, "Print plan without writing")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if *mapPath == "" || *disk == "" {
		return fmt.Errorf("--map and --disk are required")
	}
	return Write(writeOptions{MapPath: *mapPath, Disk: *disk, Confirm: *confirm, DryRun: *dryRun})
}
