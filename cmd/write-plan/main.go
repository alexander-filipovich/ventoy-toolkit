package main

import "flag"

func main() {
	mapPath := flag.String("map", "", "Path to write-map JSON")
	targetBytes := flag.Uint64("target-bytes", 0, "Target disk bytes")
	envPath := flag.String("env", "", "Path to write env output")
	rangesPath := flag.String("ranges", "", "Path to write ranges TSV")
	flag.Parse()

	if *mapPath == "" || *targetBytes == 0 || *envPath == "" || *rangesPath == "" {
		fail("--map, --target-bytes, --env and --ranges are required")
	}

	m := readMap(*mapPath)
	writePlan(m, buildLayout(m, *targetBytes), *envPath, *rangesPath)
}
