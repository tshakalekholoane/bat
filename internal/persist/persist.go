// Package persist implements the functions that are required to create
// and delete the systemd services that persist the charging threshold
// between restarts for this application.
package persist

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"text/template"

	"tshaka.co/bat/internal/file"
)

// service type holds the fields for variables that go into a systemd
// service.
type service struct {
	Event, Shell, Target string
	Threshold            int
}

// errors
var (
	errNoSuchFile      = syscall.ENOENT
	ErrBashNotFound    = errors.New("persist: bash not found")
	ErrIncompatSystemd = errors.New("persist: incompatible systemd version")
)

//go:embed unit.tmpl
var unit string

// units array contains prepopulated service structs that are used by
// systemd to support threshold persistence between various suspend or
// hibernate states.
var units = [...]service{
	{Event: "boot", Target: "multi-user"},
	{Event: "hibernation", Target: "hibernate"},
	{Event: "hybridsleep", Target: "hybrid-sleep"},
	{Event: "sleep", Target: "suspend"},
	{Event: "suspendthenhibernate", Target: "suspend-then-hibernate"},
}

// bash returns the path where the Bash shell is located. By convention
// this is either in /usr/bin/ or /bin/ and will return an error
// otherwise.
func bash() (string, error) {
	_, err := os.Stat("/usr/bin/bash")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			_, err = os.Stat("/bin/bash")
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return "", ErrBashNotFound
				}
				return "", err
			}
			return "/bin/bash", nil
		}
		return "", err
	}
	return "/usr/bin/bash", nil
}

// systemd returns true if the systemd version of the system in question
// is later than 244 and returns false otherwise. (systemd v244-rc1 is
// the earliest version to allow restarts for oneshot services).
func systemd() (bool, error) {
	cmd := exec.Command("systemctl", "--version")
	out, err := cmd.Output()
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

// DeleteServices removes all systemd services created by this
// application in order to persist the charging threshold between
// restarts.
func DeleteServices() error {
	errs := make(chan error, len(units))
	for _, s := range units {
		go func(s service) {
			err := os.Remove(
				fmt.Sprintf("/etc/systemd/system/bat-%s.service", s.Event))
			if err != nil && !errors.Is(err, errNoSuchFile) {
				errs <- err
				return
			}
			cmd := exec.Command(
				"systemctl", "disable", fmt.Sprintf("bat-%s.service", s.Event))
			var buf bytes.Buffer
			cmd.Stderr = &buf
			err = cmd.Run()
			if err != nil && !strings.Contains(
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
		err := <-errs
		if err != nil {
			return err
		}
	}
	return nil
}

// WriteServices creates all the systemd services required to persist
// the charging threshold between restarts.
func WriteServices() error {
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
	threshold, err := strconv.Atoi(strings.TrimSpace(string(limit)))
	if err != nil {
		return err
	}
	if threshold < 1 || threshold > 100 {
		log.Fatal(fmt.Errorf("persist: invalid threshold value %d", threshold))
	}
	tmpl, err := template.New("unit").Parse(unit)
	if err != nil {
		return err
	}
	errs := make(chan error, len(units))
	for _, s := range units {
		go func(s service) {
			s.Shell = shell
			s.Threshold = threshold
			f, err := os.Create(
				fmt.Sprintf("/etc/systemd/system/bat-%s.service", s.Event))
			if err != nil {
				errs <- err
				return
			}
			defer f.Close()
			err = tmpl.Execute(f, s)
			if err != nil {
				errs <- err
				return
			}
			cmd := exec.Command(
				"systemctl", "enable", fmt.Sprintf("bat-%s.service", s.Event))
			err = cmd.Run()
			if err != nil {
				errs <- err
				return
			}
			errs <- nil
		}(s)
	}
	for range units {
		err := <-errs
		if err != nil {
			return err
		}
	}
	return nil
}
