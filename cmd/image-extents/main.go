package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	image := flag.String("image", "", "Path to sparse image")
	imagePath := flag.String("image-path", "", "Image path stored in the write map")
	partitionJSON := flag.String("partition-json", "", "Path to sfdisk -J partition JSON")
	flag.Parse()

	if *image == "" {
		fail("--image is required")
	}
	if *partitionJSON == "" {
		fail("--partition-json is required")
	}
	if *imagePath == "" {
		*imagePath = *image
	}

	logicalBytes, err := fileSize(*image)
	if err != nil {
		fail(err.Error())
	}
	partitions, err := readPartitionTable(*partitionJSON)
	if err != nil {
		fail(fmt.Sprintf("partition table unavailable: %v", err))
	}
	zones, err := buildDerivedZones(partitions)
	if err != nil {
		fail(fmt.Sprintf("derived zones unavailable: %v", err))
	}

	m := writeMap{
		Schema:            "ventoy-dev-image-write-map",
		ImagePath:         *imagePath,
		ImageLogicalBytes: logicalBytes,
		PartitionTable:    partitions,
		DerivedZones:      zones,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		fail(err.Error())
	}
}

func fail(msg string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	os.Exit(1)
}
