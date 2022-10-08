// Package variable contains helpers related to reading the contents of a
// /sys/class/power_supply/BAT?/ variable.
package variable

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
)

type Variable uint8

const (
	Capacity Variable = iota + 1
	Status
	Threshold
)

func (v Variable) String() string {
	switch v {
	case Capacity:
		return "capacity"
	case Status:
		return "status"
	case Threshold:
		return "charge_control_end_threshold"
	default:
		return "unrecognised"
	}
}

// ErrNotFound indicates a virtual file that does not exist in the path
// provided.
var ErrNotFound = errors.New("variable: virtual file not found")

var dir = "/sys/class/power_supply/BAT?/"

func find(v Variable) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, v.String()))
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "", ErrNotFound
	}

	return matches[0], nil
}

// Get returns the contents of a virtual file usually located in
// /sys/class/power_supply/BAT?/ and an error otherwise.
func Get(v Variable) (string, error) {
	p, err := find(v)
	if err != nil {
		return "", nil
	}

	contents, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(contents)), nil
}

// Set writes the virtual file usually located in
// /sys/class/power_supply/BAT?/ and returns an error otherwise.
func Set(v Variable, val string) error {
	p, err := find(v)
	if err != nil {
		return err
	}

	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(val)
	return err
}
