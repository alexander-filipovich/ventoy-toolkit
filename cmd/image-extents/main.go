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
	forceFallback := flag.Bool("force-fallback", false, "Force full-range fallback (testing)")
	flag.Parse()

	if *image == "" {
		fail("--image is required")
	}
	if *imagePath == "" {
		*imagePath = *image
	}

	logicalBytes, err := fileSize(*image)
	if err != nil {
		fail(err.Error())
	}
	ranges, status, warning := scanExtents(*image, logicalBytes, *forceFallback)

	m := writeMap{
		Schema:            "ventoy-dev-image-write-map",
		ImagePath:         *imagePath,
		ImageLogicalBytes: logicalBytes,
		ExtentsStatus:     status,
		RawExtents:        ranges,
		ExpansionHole:     findExpansionHole(logicalBytes, ranges),
		Warning:           warning,
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
