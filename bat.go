package main

import (
    "errors"
    "fmt"
    "log"
    "os"
    "os/exec"
    "strconv"
    "strings"
)

var (
    help string = `                                  bat
                                    
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
    -s, --status        print charging status

REFERENCE
    https://wiki.archlinux.org/index.php/Laptop/ASUS#Battery_charge_threshold

                                13 JANUARY 2021
    `
    service string = fmt.Sprintf(
        `[Unit]
Description=Set the battery charging threshold
After=multi-user.target
StartLimitBurst=0

[Service]
Type=oneshot
Restart=on-failure
ExecStart=/bin/bash -c 'echo %s > /sys/class/power_supply/BAT0/charge_control_end_threshold'

[Install]
WantedBy=multi-user.target
        `,
        scat("/sys/class/power_supply/BAT0/charge_control_end_threshold"))
)

func scat(file string) string {
    cmd := exec.Command("cat", file)
    out, err := cmd.Output()
    if err != nil {
        log.Fatal(err)
    }
    return strings.TrimSpace(string(out))
}

func page(out string) {
    cmd := exec.Command("/usr/bin/less")
    cmd.Stdin = strings.NewReader(out)
    cmd.Stdout = os.Stdout
    err := cmd.Run()
    if err != nil {
        log.Fatal(err)
    }
}

func persist() {
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

func setThreshold(t int) {
    st := fmt.Sprintf("echo %d > "+
        "/sys/class/power_supply/BAT0/charge_control_end_threshold", t)
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

func main() {
    nArgs := len(os.Args)

    if nArgs == 1 {
        page(help)
        os.Exit(1)
    }

    switch os.Args[1] {
    case "-c", "--capacity":
        fmt.Println(scat("/sys/class/power_supply/BAT0/capacity"))
    case "-h", "--help":
        page(help)
    case "-p", "--persist":
        persist()
    case "-t", "--threshold":
        switch {
        case nArgs > 3:
            fmt.Println("Expects a single argument.")
        case nArgs == 3:
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
            fmt.Println(scat("/sys/class/power_supply/BAT0/charge_control_end_threshold"))
        }
    case "-s", "--status":
        fmt.Println(scat("/sys/class/power_supply/BAT0/status"))
    default:
        fmt.Printf("There is no %s option. Use bat --help to see a list of"+
            "available options.\n", os.Args[1])
    }
}
