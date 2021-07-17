package threshold

import (
    "errors"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"
)

// hasRequiredKernel returns true if the Linux kernel version of the
// system in question is later than 5.4 and returns false otherwise.
// (This is the earliest version of the Linux kernel to expose the
// battery charging threshold). A successful call returns err == nil.
func hasRequiredKernel() (bool, error) {
    cmd := exec.Command("uname", "--kernel-release")
    out, err := cmd.Output()
    if err != nil {
        return false, err
    }
    re := regexp.MustCompile(`\d+\.\d+`)
    ver := string(re.Find(out))
    maj, err := strconv.Atoi(strings.Split(ver, ".")[0])
    if err != nil {
        return false, err
    }
    min, err := strconv.Atoi(strings.Split(ver, ".")[1])
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
        // >= 6.0
        return true, nil
    }
    // <= 4.x
    return false, nil
}

// Write overrides the contents of the virtual file that contains the
// battery charging threshold with the given value. A successful call
// returns err == nil.
func Write(t int) error {
    ok, err := hasRequiredKernel()
    if err != nil {
        return err
    }
    if !ok {
        return errors.New("incompatible kernel version")
    }
    matches, err := filepath.Glob(
        "/sys/class/power_supply/BAT?/charge_control_end_threshold")
    if err != nil {
        return err
    } else if len(matches) == 0 {
        return errors.New("virtual file not found")
    }
    f, err := os.Create(matches[0])
    if err != nil {
        return err
    }
    defer f.Close()
    f.WriteString(fmt.Sprint(t))
    return nil
}
