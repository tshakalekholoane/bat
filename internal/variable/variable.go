// Package variable contains helpers related to reading the contents of a
// /sys/class/power_supply/BAT?/ variable.
package variable

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrNotFound indicates a virtual file that does not exist in the path
// provided.
var ErrNotFound = errors.New("variable: virtual file not found")

// Val returns the contents of a virtual file in
// /sys/class/power_supply/BAT?/ and an error otherwise.
func Val(f string) ([]byte, error) {
	matches, err := filepath.Glob("/sys/class/power_supply/BAT?/" + f)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return nil, ErrNotFound
	}

	contents, err := os.ReadFile(matches[0])
	if err != nil {
		return nil, err
	}
	return contents, nil
}
