// Package file implements a helper function that returns the value of a
// /sys/class/power_supply/BAT?/ variable.
package file

import (
	"errors"
	"os"
	"path/filepath"
)

var ErrNotFound = errors.New("file: virtual file not found")

// Contents returns the contents of a virtual file in
// /sys/class/power_supply/BAT?/ as a sequence of bytes.
func Contents(v string) ([]byte, error) {
	v = "/sys/class/power_supply/BAT?/" + v
	matches, err := filepath.Glob(v)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, ErrNotFound
	}
	f, err := os.ReadFile(matches[0])
	if err != nil {
		return nil, err
	}
	return f, nil
}
