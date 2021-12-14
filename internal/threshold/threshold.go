// Package threshold implements a function that sets the battery
// charging threshold.
package threshold

import (
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"tshaka.co/bat/internal/file"
)

var ErrIncompatKernel = errors.New("incompatible kernel version")

// kernel returns true if the Linux kernel version of the system in
// question is later than 5.4 and returns false otherwise. (This is the
// earliest version of the Linux kernel to expose the battery charging
// threshold).
func kernel() (bool, error) {
	cmd := exec.Command("uname", "--kernel-release")
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	re := regexp.MustCompile(`\d+\.\d+`)
	v := string(re.Find(out))
	maj, min, err := func(ver string) (int, int, error) {
		f, err := strconv.ParseFloat(strings.TrimSpace(ver), 64)
		if err != nil {
			return 0, 0, err
		}
		maj := int(f)
		min := (f - float64(maj)) * math.Pow10(len(strings.Split(ver, ".")[1]))
		return maj, int(min), nil
	}(v)
	if err != nil {
		return false, err
	}
	if maj >= 5 {
		if maj == 5 {
			if min >= 4 {
				return true, nil
			}
			// 5.0 < 5.4
			return false, nil
		}
		// >= 6.0 🤷‍♀️
		return true, nil
	}
	// <= 4.x
	return false, nil
}

// Set overrides the charging threshold with t.
func Set(t int) error {
	ok, err := kernel()
	if err != nil {
		return err
	}
	if !ok {
		return ErrIncompatKernel
	}
	matches, err := filepath.Glob(
		"/sys/class/power_supply/BAT?/charge_control_end_threshold")
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return file.ErrNotFound
	}
	f, err := os.Create(matches[0])
	if err != nil {
		return err
	}
	defer f.Close()
	f.WriteString(fmt.Sprint(t))
	return nil
}
