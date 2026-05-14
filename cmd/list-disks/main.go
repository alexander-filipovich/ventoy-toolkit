package main

import (
	"fmt"
	"os"
	"runtime"
)

func main() {
	if runtime.GOOS != "darwin" {
		fail("macOS only")
	}

	candidates := must(listExternalPhysicalCandidates())
	selected := must(promptSelection(candidates))

	fmt.Fprintf(os.Stderr, "%s\t%s\t%s\n", selected.DiskPath, selected.SizeHuman, selected.Name)
	fmt.Println(selected.DiskPath)
}

func must[T any](v T, err error) T {
	if err != nil {
		fail(err.Error())
	}
	return v
}

func fail(msg string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	os.Exit(1)
}
