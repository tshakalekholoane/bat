// Package test implements test cases for the application's command line
// interface.
package test

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// path returns the location of the given /sys/class/power_supply/BAT?/
// file.
func path(f string) (string, error) {
	matches, err := filepath.Glob("/sys/class/power_supply/BAT?/" + f)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("test: virtual file not found")
	}
	return matches[0], nil
}

// TestStat tests the status variables emitted by the application.
func TestStat(t *testing.T) {
	var tests = [...]struct {
		file, flag string
	}{
		{"capacity", "-c"},
		{"capacity", "--capacity"},
		{"status", "-s"},
		{"status", "--status"},
		{"charge_control_end_threshold", "-t"},
		{"charge_control_end_threshold", "--threshold"},
	}
	for _, test := range tests {
		f, err := path(test.file)
		if err != nil {
			t.Fatal(err)
		}
		got, err := exec.Command("../bat", test.flag).Output()
		if err != nil {
			t.Fatal(err)
		}
		want, err := exec.Command("cat", f).Output()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf(
				"bat %s = %s, want %s\n",
				test.flag,
				strings.TrimSpace(string(got)),
				strings.TrimSpace(string(want)))
		}
	}
}
