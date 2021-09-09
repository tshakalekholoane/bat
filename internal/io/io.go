package io

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// FileContents returns the contents of the specified (battery) virtual
// file in the sysfs pseudo file system provided by the Linux kernel. A
// successful call returns err == nil.
func FileContents(vf string) (string, error) {
	matches, err := filepath.Glob("/sys/class/power_supply/BAT?/" + vf)
	if err != nil {
		return "", err
	} else if len(matches) == 0 {
		return "", errors.New("virtual file not found")
	}
	f, err := os.ReadFile(matches[0])
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(f)), nil
}
