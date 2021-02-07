package main

import (
    "errors"
    "fmt"
    "log"
    "os"
    "os/exec"
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
    -c, --capacity,     print current battery level
    -h, --help,         print this help document
    -p, --persist       persist the current threshold setting between restarts
                        (requires sudo permissions)
    -t, --threshold,    print the current charging threshold limit
                        append a value between 1 and 100 to set a new threshold
                        e.g bat --threshold 80
    -s, --status        print charging status

REFERENCE
    https://wiki.archlinux.org/index.php/Laptop/ASUS#Battery_charge_threshold

                                13 JANUARY 2021
`

// scat returns a string of a file's contents.
func scat(path string) string {
    cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("cat %s", path))
    out, err := cmd.Output()
    if err != nil {
        log.Fatal(err)
    }
    return strings.TrimSpace(string(out))
}

// page invokes the less pager on a specified string.
func page(out string) {
    cmd := exec.Command("/bin/less")
    cmd.Stdin = strings.NewReader(out)
    cmd.Stdout = os.Stdout
    err := cmd.Run()
    if err != nil {
        log.Fatal(err)
    }
}

// persist persists the prevailing battery charging threshold level
// between restarts by creating or updating a systemd service with the
// name `bat.service`.
func persist() {
    service := fmt.Sprintf(
        `[Unit]
Description=Set the battery charging threshold
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
    f, err := os.Create("/etc/systemd/system/bat.service")
    if err != nil {
        if strings.HasSuffix(err.Error(), ": permission denied") {
            fmt.Println("This command requires running with sudo.")
            os.Exit(1)
        } else {
            log.Fatal(err)
        }
    }
    defer f.Close()
    f.WriteString(service)
    cmd := exec.Command("systemctl", "enable", "bat.service")
    err = cmd.Run()
    if err != nil {
        log.Fatal(err)
    }
}

// setThreshold sets the charging threshold by writing to the
// `charge_control_end_threshold` file after gaining root user
// permissions and prints a message to the terminal about the status of
// the operation.
func setThreshold(t int) {
    st := fmt.Sprintf("echo %d > "+
        "/sys/class/power_supply/BAT?/charge_control_end_threshold", t)
    cmd := exec.Command("su", "-c", st)
    fmt.Print("Root password: ")
    cmd.Stdin = os.Stdin
    err := cmd.Run()
    if err != nil {
        fmt.Println("\rAuthentication failure.")
        os.Exit(1)
    }
    fmt.Printf("\rCharging threshold set to %d.\n", t)
}

// hasRequiredKernelVer returns true if the Linux kernel version of the
// system in question is later than 5.4 and returns false otherwise.
func hasRequiredKernelVer() bool {
    cmd := exec.Command("uname", "-r")
    out, err := cmd.Output()
    if err != nil {
        log.Fatal(err)
    }
    re := regexp.MustCompile(`[0-9]+\.[0-9]+`)
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
    case "-t", "--threshold":
        switch {
        case n > 3:
            fmt.Println("Expects a single argument.")
        case n == 3:
            t, err := strconv.Atoi(os.Args[2])
            if err != nil {
                if errors.Is(err, strconv.ErrSyntax) {
                    fmt.Println("Argument should be an integer.")
                    os.Exit(1)
                } else {
                    log.Fatal(err)
                }
            }
            if t < 1 || t > 100 {
                fmt.Println("Number should be between 1 and 100.")
                os.Exit(1)
            }
            setThreshold(t)
        default:
            fmt.Println(scat("/sys/class/power_supply/BAT?/charge_control_end_threshold"))
        }
    case "-s", "--status":
        fmt.Println(scat("/sys/class/power_supply/BAT?/status"))
    default:
        fmt.Printf("There is no %s option. Use bat --help to see a list of"+
            "available options.\n", os.Args[1])
    }
}
