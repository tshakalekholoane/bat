package main

import (
    "bytes"
    "errors"
    "fmt"
    "log"
    "os"
    "os/exec"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"
)

// help documentation.
var help string = `                                  bat

NAME
    bat - battery management utility for Linux laptops

SYNOPSIS
    bat [OPTION]

DESCRIPTION
    -c, --capacity      print the current battery level
    -h, --help          print this help document
    -p, --persist       persist the current charging threshold setting between
                        restarts (requires superuser permissions)
    -r, --reset         prevents the charging threshold from persisting between
                        restarts
    -s, --status        print charging status
    -t, --threshold     print the current charging threshold limit
                        specify a value between 1 and 100 to set a new threshold
                        (the latter requires superuser permissions)
                        e.g. bat --threshold 80

REFERENCE
    https://wiki.archlinux.org/index.php/Laptop/ASUS#Battery_charge_threshold

                                13 JANUARY 2021
`

// hasRequiredKernelVer returns true if the Linux kernel version of the
// system in question is later than 5.4 and returns false otherwise.
func hasRequiredKernelVer() bool {
    cmd := exec.Command("uname", "-r")
    out, err := cmd.Output()
    if err != nil {
        log.Fatal(err)
    }
    re := regexp.MustCompile(`\d+\.\d+`)
    ver := string(re.Find(out))
    maj, _ := strconv.Atoi(strings.Split(ver, ".")[0])
    min, _ := strconv.Atoi(strings.Split(ver, ".")[1])
    if maj >= 5 {
        if maj == 5 {
            if min >= 4 {
                return true
            }
            // Minor version < 4.
            return false
        }
        // Major version > 5.
        return true
    }
    // Major version < 5.
    return false
}

// hasRequiredSystemdVer returns true if the systemd version of the
// system in question is later than 244 and returns false otherwise.
func hasRequiredSystemdVer() bool {
    cmd := exec.Command("systemctl", "--version")
    out, err := cmd.Output()
    if err != nil {
        log.Fatal(err)
    }
    re := regexp.MustCompile(`\d+`)
    ver, _ := strconv.Atoi(string(re.Find(out)))
    if ver < 244 {
        return false
    }
    return true
}

// page invokes the less pager on a specified string.
func page(out string) {
    cmd := exec.Command("less")
    cmd.Stdin = strings.NewReader(out)
    cmd.Stdout = os.Stdout
    err := cmd.Run()
    if err != nil {
        log.Fatal(err)
    }
}

// persist persists the prevailing battery charging threshold level
// between restarts and hibernation by creating or updating the systemd
// services `bat-boot.service` and `bat-sleep.service`.
func persist() {
    if !hasRequiredSystemdVer() {
        fmt.Println("Requires systemd version 244 or later.")
        os.Exit(1)
    }

    // Write systemd service that will persist the threshold after
    // restarts.
    bootUnit := fmt.Sprintf(
        `[Unit]
Description=Persist the battery charging threshold between restarts
After=multi-user.target
StartLimitBurst=0

[Service]
Type=oneshot
Restart=on-failure
ExecStart=/bin/bash -c 'echo %s > /sys/class/power_supply/BAT?/charge_control_end_threshold'

[Install]
WantedBy=multi-user.target
        `,
        scat("/sys/class/power_supply/BAT?/charge_control_end_threshold"))
    bootService, err := os.Create("/etc/systemd/system/bat-boot.service")
    if err != nil {
        if strings.HasSuffix(err.Error(), ": permission denied") {
            fmt.Println("This command requires sudo permissions.")
            os.Exit(1)
        }
        log.Fatal(err)
    }
    defer bootService.Close()
    bootService.WriteString(bootUnit)

    // Enable the service.
    cmd := exec.Command("systemctl", "enable", "bat-boot.service")
    err = cmd.Run()
    if err != nil {
        log.Fatal(err)
    }

    // Write systemd service that will persist the threshold after
    // hibernation.
    sleepUnit := fmt.Sprintf(
        `[Unit]
Description=Persist the battery charging threshold after hibernation 
Before=sleep.target
StartLimitBurst=0

[Service]
Type=oneshot
Restart=on-failure
ExecStart=/bin/bash -c 'echo %s > /sys/class/power_supply/BAT?/charge_control_end_threshold'

[Install]
WantedBy=sleep.target
        `,
        scat("/sys/class/power_supply/BAT?/charge_control_end_threshold"))
    sleepService, err := os.Create("/etc/systemd/system/bat-sleep.service")
    if err != nil {
        log.Fatal(err)
    }
    defer sleepService.Close()
    sleepService.WriteString(sleepUnit)

    // Enable the service.
    cmd = exec.Command("systemctl", "enable", "bat-sleep.service")
    err = cmd.Run()
    if err != nil {
        log.Fatal(err)
    }
}

// reset disables the systemd service that persists the charging
// threshold between restarts and hibernation.
func reset() {
    // Delete service that persists the charging threshold between
    // restarts.
    err := os.Remove("/etc/systemd/system/bat-boot.service")
    if err != nil {
        switch {
        case strings.HasSuffix(err.Error(), ": permission denied"):
            fmt.Println("This command requires sudo permissions.")
            os.Exit(1)
        case strings.HasSuffix(err.Error(), ": no such file or directory"):
            break
        default:
            log.Fatal(err)
        }
    }
    cmd := exec.Command("systemctl", "disable", "bat-boot.service")
    var stdErr bytes.Buffer
    cmd.Stderr = &stdErr
    err = cmd.Run()
    if err != nil {
        msg := strings.TrimSpace(stdErr.String())
        if !strings.HasSuffix(msg, " file bat-boot.service does not exist.") {
            log.Fatal(err)
        }
    }

    // Delete the service that persists the charging threshold after
    // hibernation.
    err = os.Remove("/etc/systemd/system/bat-sleep.service")
    if err != nil {
        if !strings.HasSuffix(err.Error(), ": no such file or directory") {
            log.Fatal(err)
        }
    }
    cmd = exec.Command("systemctl", "disable", "bat-sleep.service")
    cmd.Stderr = &stdErr
    err = cmd.Run()
    if err != nil {
        msg := strings.TrimSpace(stdErr.String())
        if !strings.HasSuffix(msg, " file bat-sleep.service does not exist.") {
            log.Fatal(err)
        }
    }
}

// scat returns a string of a file's contents.
func scat(path string) string {
    cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("cat %s", path))
    out, err := cmd.Output()
    if err != nil {
        log.Fatal(err)
    }
    return strings.TrimSpace(string(out))
}

// setThreshold sets the charging threshold by writing to the
// `charge_control_end_threshold` variable after gaining superuser
// permissions and prints a message to the terminal about the status of
// the operation.
func setThreshold(threshold int) {
    files, err := filepath.Glob("/sys/class/power_supply/BAT?/charge_control_end_threshold")
    if err != nil {
        log.Fatal(err)
    }
    if len(files) == 0 {
        fmt.Println("This program is most likely not compatible with your system.")
        fmt.Println("See https://github.com/leveson/bat#disclaimer for details.")
        os.Exit(1)
    }
    f, err := os.Create(files[0])
    if err != nil {
        if strings.HasSuffix(err.Error(), ": permission denied") {
            fmt.Println("This command requires sudo permissions.")
            os.Exit(1)
        }
        log.Fatal(err)
    }
    defer f.Close()
    f.WriteString(fmt.Sprint(threshold))
    fmt.Printf("\rCharging threshold set to %d.\n", threshold)
}

func main() {
    if !hasRequiredKernelVer() {
        fmt.Println("Requires Linux kernel version 5.4 or later.")
        os.Exit(1)
    }

    n := len(os.Args)
    if n == 1 {
        page(help)
        os.Exit(1)
    }

    switch os.Args[1] {
    case "-c", "--capacity":
        fmt.Println(scat("/sys/class/power_supply/BAT?/capacity"))
    case "-h", "--help":
        page(help)
    case "-p", "--persist":
        persist()
    case "-r", "--reset":
        reset()
    case "-s", "--status":
        fmt.Println(scat("/sys/class/power_supply/BAT?/status"))
    case "-t", "--threshold":
        switch {
        case n > 3:
            fmt.Println("Expects a single argument.")
        case n == 3:
            threshold, err := strconv.Atoi(os.Args[2])
            if err != nil {
                if errors.Is(err, strconv.ErrSyntax) {
                    fmt.Println("Argument should be an integer.")
                    os.Exit(1)
                } else {
                    log.Fatal(err)
                }
            }
            if threshold < 1 || threshold > 100 {
                fmt.Println("Number should be between 1 and 100.")
                os.Exit(1)
            }
            setThreshold(threshold)
        default:
            fmt.Println(scat("/sys/class/power_supply/BAT?/charge_control_end_threshold"))
        }
    default:
        fmt.Printf(
            "There is no %s option. Use bat --help to see a list of available options.\n",
            os.Args[1])
    }
}
