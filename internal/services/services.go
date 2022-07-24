// Package services implements the functions that are required to create
// and delete the systemd services that persist the charging threshold
// between restarts for this application.
package services

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"text/template"

	"tshaka.co/bat/internal/file"
	"tshaka.co/bat/internal/threshold"
)

// service type holds the fields for variables that go into a systemd
// service.
type service struct {
	Event, Shell, Target string
	Threshold            int
}

var (
	// ErrBashNotFound indicates the absence of the Bash shell in the
	// user's $PATH.
	ErrBashNotFound = errors.New("persist: Bash not found")
	// ErrIncompatSystemd indicates an incompatible version of systemd.
	ErrIncompatSystemd = errors.New("persist: incompatible systemd version")
)

//go:embed unit.tmpl
var unit string

// units array contains populated service structs that are used by
// systemd to support threshold persistence between various suspend or
// hibernate states.
var units = [...]service{
	{Event: "boot", Target: "multi-user"},
	{Event: "hibernation", Target: "hibernate"},
	{Event: "hybridsleep", Target: "hybrid-sleep"},
	{Event: "sleep", Target: "suspend"},
	{Event: "suspendthenhibernate", Target: "suspend-then-hibernate"},
}

// bash returns the path where the Bash shell is located.
func bash() (string, error) {
	path, err := exec.LookPath("bash")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", ErrBashNotFound
		}
		return "", err
	}
	return path, nil
}

// systemd returns true if the systemd version of the system in question
// is later than 244 and returns false otherwise. (systemd v244-rc1 is
// the earliest version to allow restarts for oneshot services).
func systemd() (bool, error) {
	out, err := exec.Command("systemctl", "--version").Output()
	if err != nil {
		return false, err
	}

	re := regexp.MustCompile(`\d+`)
	ver, err := strconv.Atoi(string(re.Find(out)))
	if err != nil {
		return false, err
	}

	if ver < 244 {
		return false, nil
	}
	return true, nil
}

// Delete removes all systemd services created by this
// application in order to persist the charging threshold between
// restarts.
func Delete() error {
	errs := make(chan error, len(units))
	for _, s := range units {
		go func(s service) {
			err := os.Remove(fmt.Sprintf("/etc/systemd/system/bat-%s.service", s.Event))
			if err != nil && !errors.Is(err, syscall.ENOENT /* no such file */) {
				errs <- err
				return
			}

			cmd := exec.Command("systemctl", "disable", fmt.Sprintf("bat-%s.service", s.Event))
			var buf bytes.Buffer
			cmd.Stderr = &buf
			if err := cmd.Run(); err != nil && !strings.Contains(
				strings.TrimSpace(buf.String()),
				fmt.Sprintf("bat-%s.service does not exist.", s.Event),
			) {
				errs <- err
				return
			}
			errs <- nil
		}(s)
	}
	for range units {
		if err := <-errs; err != nil {
			return err
		}
	}
	return nil
}

// Write creates all the systemd services required to persist
// the charging threshold between restarts.
func Write() error {
	ok, err := systemd()
	if err != nil {
		return err
	}
	if !ok {
		return ErrIncompatSystemd
	}
	shell, err := bash()
	if err != nil {
		return err
	}
	limit, err := file.Contents("charge_control_end_threshold")
	if err != nil {
		return err
	}
	t, err := strconv.Atoi(strings.TrimSpace(string(limit)))
	if err != nil {
		return err
	}
	if !threshold.IsValid(t) {
		log.Fatalf("persist: invalid threshold value %d\n", t)
	}
	tmpl, err := template.New("unit").Parse(unit)
	if err != nil {
		return err
	}
	errs := make(chan error, len(units))
	for _, s := range units {
		go func(s service) {
			s.Shell, s.Threshold = shell, t

			f, err := os.Create(fmt.Sprintf("/etc/systemd/system/bat-%s.service", s.Event))
			if err != nil {
				errs <- err
				return
			}
			defer f.Close()

			if err := tmpl.Execute(f, s); err != nil {
				errs <- err
				return
			}

			cmd := exec.Command("systemctl", "enable", fmt.Sprintf("bat-%s.service", s.Event))
			if err := cmd.Run(); err != nil {
				errs <- err
				return
			}
			errs <- nil
		}(s)
	}

	for range units {
		if err := <-errs; err != nil {
			return err
		}
	}
	return nil
}
