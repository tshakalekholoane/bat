// Package power contains functions to read and write
// /sys/class/power_supply/ device variables.
package power

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
)

// Variable represents a /sys/class/power_supply/ device variable.
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

// dir is the location of the (symlink) of the device in the sysfs
// virtual file system. A glob pattern is used to try to make compatible
// with multiple device manufacturers.
var dir = "/sys/class/power_supply/BAT?/"

// ErrNotFound indicates a virtual file that does not exist in the path
// provided.
var ErrNotFound = errors.New("power: virtual file not found")

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
