package host

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func RunOut(name string, args ...string) ([]byte, error) {
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

func RunInherit(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}

func RunPlistJSON(name string, args ...string) ([]byte, error) {
	plistOut, err := RunOut(name, args...)
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
