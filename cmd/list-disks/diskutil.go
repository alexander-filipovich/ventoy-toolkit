package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func runPlistAsJSON(name string, args ...string) ([]byte, error) {
	plistOut, err := run(name, args...)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("plutil", "-convert", "json", "-o", "-", "-")
	cmd.Stdin = bytes.NewReader(plistOut)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	jsonOut, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}
	return jsonOut, nil
}

// run executes a command and returns stdout.
// If the command fails and wrote to stderr, we surface that text as the error.
func run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}
	return out, nil
}
