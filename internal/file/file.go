// Package file contains helpers related to reading the contents of a
// /sys/class/power_supply/BAT?/ variable.
package file

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrNotFound indicates a virtual file that does not exist in the path
// provided.
var ErrNotFound = errors.New("file: virtual file not found")

// Contents returns the contents of a virtual file in
// /sys/class/power_supply/BAT?/ as a slice of bytes.
func Contents(f string) ([]byte, error) {
	matches, err := filepath.Glob("/sys/class/power_supply/BAT?/" + f)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return nil, ErrNotFound
	}

	val, err := os.ReadFile(matches[0])
	if err != nil {
		return nil, err
	}
	return val, nil
}
