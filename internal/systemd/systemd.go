// Package systemd implements functions that relate to managing the
// systemd services that persist the charging threshold between
// restarts.
package systemd

import (
	"bytes"
	_ "embed"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"syscall"
	"text/template"

	"tshaka.co/x/bat/pkg/power"
)

var (
	// ErrBashNotFound indicates the absence of the Bash shell in the
	// user's $PATH. This is the shell program that is used to execute
	// commands to set the threshold after restarts.
	ErrBashNotFound = errors.New("systemd: Bash not found")
	// ErrIncompatSystemd indicates an incompatible version of systemd.
	ErrIncompatSystemd = errors.New("services: incompatible systemd version")
)

// unit is a template of a systemd unit file that encodes information
// about the services used to persist the charging threshold between
// restarts.
//
//go:embed unit.tmpl
var unit string

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

// config represents a systemd unit file's configuration for a service.
type config struct {
	Event, Shell, Target string
	Threshold            int
}

func configs() ([]config, error) {
	shell, err := bash()
	if err != nil {
		return nil, err
	}

	val, err := power.Get(power.Threshold)
	if err != nil {
		return nil, err
	}

	threshold, err := strconv.Atoi(val)
	if err != nil {
		return nil, err
	}

	return []config{
		{"boot", shell, "multi-user", threshold},
		{"hibernation", shell, "hibernate", threshold},
		{"hybridsleep", shell, "hybrid-sleep", threshold},
		{"sleep", shell, "suspend", threshold},
		{"suspendthenhibernate", shell, "suspend-then-hibernate", threshold},
	}, nil
}

type Systemd struct {
	dir string
}

func New() *Systemd { return &Systemd{dir: "/etc/systemd/system/"} }

func (s *Systemd) remove(configs []config) error {
	errs := make(chan error, len(configs))
	for _, c := range configs {
		go func(c config) {
			name := "bat-" + c.Event + ".service"

			err := os.Remove(s.dir + name)
			if err != nil && !errors.Is(err, syscall.ENOENT /* no such file */) {
				errs <- err
				return
			}

			errs <- nil
		}(c)
	}

	for range configs {
		if err := <-errs; err != nil {
			return err
		}
	}

	return nil
}

func (s *Systemd) write(configs []config) error {
	ok, err := systemd()
	if err != nil {
		return err
	}
	if !ok {
		return ErrIncompatSystemd
	}

	tmpl, err := template.New("unit").Parse(unit)
	if err != nil {
		return err
	}

	errs := make(chan error, len(configs))
	for _, c := range configs {
		go func(c config) {
			name := "bat-" + c.Event + ".service"

			service, err := os.Create(s.dir + name)
			if err != nil && !errors.Is(err, syscall.ENOENT /* no such file */) {
				errs <- err
				return
			}
			defer service.Close()

			if err := tmpl.Execute(service, c); err != nil {
				errs <- err
				return
			}

			errs <- nil
		}(c)
	}

	for range configs {
		if err := <-errs; err != nil {
			return err
		}
	}

	return nil
}

func (s *Systemd) disable(configs []config) error {
	errs := make(chan error, len(configs))
	for _, c := range configs {
		go func(c config) {
			name := "bat-" + c.Event + ".service"

			cmd := exec.Command("systemctl", "disable", name)
			buf := new(bytes.Buffer)
			cmd.Stderr = buf

			if err := cmd.Run(); err != nil && !bytes.Contains(buf.Bytes(), []byte(name+" does not exist.")) {
				errs <- err
				return
			}
			errs <- nil
		}(c)
	}

	for range configs {
		if err := <-errs; err != nil {
			return err
		}
	}

	return nil
}

func (s *Systemd) enable(configs []config) error {
	errs := make(chan error, len(configs))
	for _, c := range configs {
		go func(c config) {
			name := "bat-" + c.Event + ".service"

			cmd := exec.Command("systemctl", "enable", name)
			if err := cmd.Run(); err != nil {
				errs <- err
				return
			}
			errs <- nil
		}(c)
	}

	for range configs {
		if err := <-errs; err != nil {
			return err
		}
	}

	return nil
}

// Reset removes and disables all systemd services created by the
// application.
func (s *Systemd) Reset() error {
	configs, err := configs()
	if err != nil {
		return err
	}

	if err := s.remove(configs); err != nil {
		return err
	}

	if err := s.disable(configs); err != nil {
		return err
	}

	return nil
}

// Write creates all the systemd services required to persist the
// charging threshold between restarts.
func (s *Systemd) Write() error {
	configs, err := configs()
	if err != nil {
		return err
	}

	if err := s.write(configs); err != nil {
		return err
	}

	if err := s.enable(configs); err != nil {
		return err
	}

	return nil
}
