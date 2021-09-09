package persist

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/tshakalekholoane/bat/internal/io"
)

//go:embed unit.tmpl
var unit string

// units array contains prepopulated service structs that are used by
// systemd to support threshold persistence between various suspend or
// hibernate states.
var units = [...]struct {
	Event     string
	Shell     string
	Target    string
	Threshold string
}{
	{Event: "boot", Target: "multi-user"},
	{Event: "hibernation", Target: "hibernate"},
	{Event: "hybridsleep", Target: "hybrid-sleep"},
	{Event: "sleep", Target: "suspend"},
	{Event: "suspendthenhibernate", Target: "suspend-then-hibernate"},
}

// bashLocation returns the location of the Bash shell as a string. A
// successful call returns err == nil. It will return the first instance
// found starting by searching in /usr/bin/ and then in /bin/ as a last
// resort.
func bashLocation() (string, error) {
	_, err := os.Stat("/usr/bin/bash")
	if err != nil {
		if os.IsNotExist(err) {
			_, err = os.Stat("/bin/bash")
			if err != nil {
				if os.IsNotExist(err) {
					return "", errors.New("bash not found")
				}
				return "", err
			}
			return "/bin/bash", nil
		}
		return "", err
	}
	return "/usr/bin/bash", nil
}

// hasRequiredSystemd returns true if the systemd version of the system
// in question is later than 244 and returns false otherwise. (systemd
// v244-rc1 is the earliest version to allow restarts for oneshot
// services). A successful call returns err == nil.
func hasRequiredSystemd() (bool, error) {
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

// RemoveServices removes the systemd services, `bat-boot.service` and
// `bat-sleep.service` that are used to persist the charging threshold
// level between restarts. If the call is successful, the return value
// is nil.
func RemoveServices() error {
	for _, service := range units {
		err := os.Remove(
			fmt.Sprintf("/etc/systemd/system/bat-%s.service", service.Event))
		if err != nil {
			switch {
			case strings.HasSuffix(err.Error(), "no such file or directory"):
				break
			default:
				return err
			}
		}
		cmd := exec.Command(
			"systemctl",
			"disable",
			fmt.Sprintf("bat-%s.service", service.Event))
		var stdErr bytes.Buffer
		cmd.Stderr = &stdErr
		err = cmd.Run()
		if err != nil {
			if !strings.HasSuffix(
				strings.TrimSpace(stdErr.String()),
				fmt.Sprintf("bat-%s.service does not exist.", service.Event)) {
				return err
			}
		}
	}
	return nil
}

// WriteServices creates the two systemd services, `bat-boot.service`
// and `bat-sleep.service` that are required to persist the charging
// threshold level between restarts. If the call is successful, the
// return value is nil.
func WriteServices() error {
	ok, err := hasRequiredSystemd()
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("incompatible systemd version")
	}
	shell, err := bashLocation()
	if err != nil {
		return err
	}
	threshold, err := io.FileContents("charge_control_end_threshold")
	if err != nil {
		return err
	}
	tmpl, err := template.New("unit").Parse(unit)
	if err != nil {
		return err
	}
	for _, service := range units {
		service.Shell = shell
		service.Threshold = threshold
		f, err := os.Create(
			fmt.Sprintf("/etc/systemd/system/bat-%s.service", service.Event))
		if err != nil {
			return err
		}
		defer f.Close()
		err = tmpl.Execute(f, service)
		if err != nil {
			return err
		}
		cmd := exec.Command(
			"systemctl",
			"enable",
			fmt.Sprintf("bat-%s.service", service.Event))
		err = cmd.Run()
		if err != nil {
			return err
		}
	}
	return nil
}
