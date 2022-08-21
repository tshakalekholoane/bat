// Package threshold implements a function that sets the battery
// charging threshold.
package threshold

import (
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"tshaka.co/bat/internal/variable"
)

// threshold represents the path of the charging threshold threshold.
var threshold = "/sys/class/power_supply/BAT?/charge_control_end_threshold"

// ErrIncompatKernel indicates an incompatible Linux kernel version.
var ErrIncompatKernel = errors.New("threshold: incompatible kernel version")

// isRequiredKernel returns true if the string ver represents a
// semantic version later than 5.4 and false otherwise (this is the
// earliest version of the Linux kernel to expose the battery charging
// threshold variable). It also returns an error if it failed parse the
// string.
func isRequiredKernel(ver string) (bool, error) {
	re := regexp.MustCompile(`\d+\.\d+`)
	ver = re.FindString(ver)
	maj, min, err := func(ver string) (int, int, error) {
		f, err := strconv.ParseFloat(strings.TrimSpace(ver), 64)
		if err != nil {
			return 0, 0, err
		}
		maj := int(f)
		min := (f - float64(maj)) * math.Pow10(len(strings.Split(ver, ".")[1]))
		return maj, int(min), nil
	}(ver)
	if err != nil {
		return false, err
	}

	if maj > 5 /* 🤷 */ || (maj == 5 && min >= 4) {
		return true, nil
	}
	return false, nil
}

// kernel returns the Linux kernel version as a string and an error
// otherwise.
func kernel() (string, error) {
	cmd := exec.Command("uname", "--kernel-release")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// IsValid returns true if v is in the range 1..=100.
func IsValid(v int) bool {
	return v >= 1 && v <= 100
}

// Set overrides the charging threshold with t.
func Set(t int) error {
	ver, err := kernel()
	if err != nil {
		return err
	}
	ok, err := isRequiredKernel(ver)
	if err != nil {
		return err
	}
	if !ok {
		return ErrIncompatKernel
	}

	matches, err := filepath.Glob(threshold)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return variable.ErrNotFound
	}

	f, err := os.Create(matches[0])
	if err != nil {
		return err
	}
	defer f.Close()

	f.WriteString(strconv.FormatInt(int64(t), 10))
	return nil
}
