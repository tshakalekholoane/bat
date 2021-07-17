package test

import (
    "bytes"
    "errors"
    "os/exec"
    "path/filepath"
    "strings"
    "testing"
)

// filepath_ returns the /sys/class/power_supply/BAT?/* file `f`. A
// successful call returns err == nil.
func filepath_(f string) (string, error) {
    matches, err := filepath.Glob("/sys/class/power_supply/BAT?/" + f)
    if err != nil {
        return "", err
    } else if len(matches) == 0 {
        return "", errors.New("virtual file not found")
    }
    return matches[0], nil
}

// TestCapacity tests the --capacity flag of the bat program by checking
// for a valid return value.
func TestCapacity(t *testing.T) {
    f, err := filepath_("capacity")
    if err != nil {
        t.Fatal(err)
    }
    got, err := exec.Command("../bat", "--capacity").Output()
    if err != nil {
        t.Fatal(err)
    }
    want, err := exec.Command("cat", f).Output()
    if err != nil {
        t.Fatal(err)
    }
    if !bytes.Equal(got, want) {
        t.Fatalf(
            "--capacity, want: %v, got: %v",
            strings.TrimSpace(string(want)),
            strings.TrimSpace(string(got)))
    }
}

// TestStatus tests the --status flag of the bat program by checking
// for a valid return value.
func TestStatus(t *testing.T) {
    f, err := filepath_("status")
    if err != nil {
        t.Fatal(err)
    }
    got, err := exec.Command("../bat", "--status").Output()
    if err != nil {
        t.Fatal(err)
    }
    want, err := exec.Command("cat", f).Output()
    if err != nil {
        t.Fatal(err)
    }
    if !bytes.Equal(got, want) {
        t.Fatalf(
            "--status, want: %v, got: %v",
            strings.TrimSpace(string(want)),
            strings.TrimSpace(string(got)))
    }
}

// TestThreshold tests the --threshold flag of the bat program by checking
// for a valid return value.
func TestThreshold(t *testing.T) {
    f, err := filepath_("charge_control_end_threshold")
    if err != nil {
        t.Fatal(err)
    }
    got, err := exec.Command("../bat", "--threshold").Output()
    if err != nil {
        t.Fatal(err)
    }
    want, err := exec.Command("cat", f).Output()
    if err != nil {
        t.Fatal(err)
    }
    if !bytes.Equal(got, want) {
        t.Fatalf(
            "--threshold, want: %v, got: %v",
            strings.TrimSpace(string(want)),
            strings.TrimSpace(string(got)))
    }
}
