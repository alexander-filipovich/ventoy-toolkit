package main

import (
	"fmt"
	"os"
)

func mustCreate(path string) *os.File {
	f, err := os.Create(path)
	if err != nil {
		fail(err.Error())
	}
	return f
}

func mustClose(f *os.File) {
	if err := f.Close(); err != nil {
		fail(err.Error())
	}
}

func writeEnv(f *os.File, key, value string) {
	for _, r := range value {
		if r == '\n' || r == '\'' {
			fail("env value contains unsupported character")
		}
	}
	if _, err := fmt.Fprintf(f, "%s='%s'\n", key, value); err != nil {
		fail(err.Error())
	}
}

func fail(msg string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	os.Exit(1)
}
