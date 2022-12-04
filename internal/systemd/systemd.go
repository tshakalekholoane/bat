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
	ErrIncompatSystemd = errors.New("systemd: incompatible systemd version")
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

// compatSystemd returns nil if the systemd version of the system in
// question is later than 244 and returns false otherwise.
// (systemd v244-rc1 is the earliest version to allow restarts for
// oneshot services).
func compatSystemd() error {
	out, err := exec.Command("systemctl", "--version").Output()
	if err != nil {
		return err
	}

	re := regexp.MustCompile(`\d+`)
	ver, err := strconv.Atoi(string(re.Find(out)))
	if err != nil {
		return err
	}

	if ver < 244 {
		return ErrIncompatSystemd
	}

	return nil
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

type Systemd struct{ dir string }

func New() *Systemd { return &Systemd{dir: "/etc/systemd/system/"} }

// sync runs the given function on the configurations in parallel and
// returns an error if any one call resulted in a error.
func sync(cfgs []config, fn func(cfg config, in chan<- error)) error {
	errs := make(chan error, len(cfgs))
	for _, cfg := range cfgs {
		go fn(cfg, errs)
	}

	for range cfgs {
		if err := <-errs; err != nil {
			return err
		}
	}

	return nil
}

func (s *Systemd) remove(cfgs []config) error {
	return sync(cfgs, func(cfg config, in chan<- error) {
		name := s.dir + "bat-" + cfg.Event + ".service"
		if err := os.Remove(name); err != nil && errors.Is(err, syscall.ENOENT) {
			in <- err
			return
		}
		in <- nil
	})
}

func (s *Systemd) write(cfgs []config) error {
	if err := compatSystemd(); err != nil {
		return err
	}

	tmpl, err := template.New("unit").Parse(unit)
	if err != nil {
		return err
	}

	return sync(cfgs, func(cfg config, in chan<- error) {
		name := s.dir + "bat-" + cfg.Event + ".service"
		sf, err := os.Create(name)
		if err != nil && !errors.Is(err, syscall.ENOENT) {
			in <- err
			return
		}
		defer sf.Close()

		if err := tmpl.Execute(sf, cfg); err != nil {
			in <- err
			return
		}
		in <- nil
	})
}

func (s *Systemd) disable(cfgs []config) error {
	return sync(cfgs, func(cfg config, in chan<- error) {
		name := "bat-" + cfg.Event + ".service"
		buf := new(bytes.Buffer)

		cmd := exec.Command("systemctl", "disable", name)
		cmd.Stderr = buf
		if err := cmd.Run(); err != nil &&
			!bytes.Contains(buf.Bytes(), []byte(name+" does not exist.")) {
			in <- err
			return
		}
		in <- nil
	})
}

func (s *Systemd) enable(cfgs []config) error {
	return sync(cfgs, func(cfg config, in chan<- error) {
		name := "bat-" + cfg.Event + ".service"
		cmd := exec.Command("systemctl", "enable", name)
		if err := cmd.Run(); err != nil {
			in <- err
			return
		}
		in <- nil
	})
}

// Reset removes and disables all systemd services created by the
// application.
func (s *Systemd) Reset() error {
	cfgs, err := configs()
	if err != nil {
		return err
	}

	if err := s.remove(cfgs); err != nil {
		return err
	}

	if err := s.disable(cfgs); err != nil {
		return err
	}

	return nil
}

// Write creates all the systemd services required to persist the
// charging threshold between restarts.
func (s *Systemd) Write() error {
	cfgs, err := configs()
	if err != nil {
		return err
	}

	if err := s.write(cfgs); err != nil {
		return err
	}

	if err := s.enable(cfgs); err != nil {
		return err
	}

	return nil
}
