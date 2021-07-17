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

    "github.com/leveson/bat/internal/io"
)

//go:embed unit.tmpl
var unit string

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
    for _, service := range [2]string{"boot", "sleep"} {
        err := os.Remove(
            fmt.Sprintf("/etc/systemd/system/bat-%s.service", service))
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
            fmt.Sprintf("bat-%s.service", service))
        var stdErr bytes.Buffer
        cmd.Stderr = &stdErr
        err = cmd.Run()
        if err != nil {
            if !strings.HasSuffix(
                strings.TrimSpace(stdErr.String()),
                fmt.Sprintf("file bat-%s.service does not exist.", service)) {
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
    threshold, err := io.FileContents("charge_control_end_threshold")
    if err != nil {
        return err
    }
    units := [2]Service{
        {"boot", "multi-user", threshold},
        {"sleep", "suspend", threshold},
    }
    tmpl, err := template.New("unit").Parse(unit)
    if err != nil {
        return err
    }
    for _, service := range units {
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
