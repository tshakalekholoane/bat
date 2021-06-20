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
    "text/template"
)

// fileContents takes a file path and returns it's contents as a string.
func fileContents(path string) string {
    bat, err := filepath.Glob(path)
    if len(bat) == 0 {
        fmt.Println("This program is most likely not compatible with your " +
            "system. See\nhttps://github.com/leveson/bat#disclaimer.")
        os.Exit(1)
    }
    contents, err := os.ReadFile(bat[0])
    if err != nil {
        log.Fatal(err)
    }
    return strings.TrimSpace(string(contents))
}

// hasRequiredKernelVer returns true if the Linux kernel version of the
// system in question is later than 5.4 and returns false otherwise.
// (This is the earliest version of the Linux kernel to expose the
// battery charging threshold).
func hasRequiredKernelVer() bool {
    cmd := exec.Command("uname", "--kernel-release")
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
            // 5.0 < 5.4
            return false
        }
        // >= 6.0
        return true
    }
    // <= 4.x
    return false
}

// hasRequiredSystemdVer returns true if the systemd version of the
// system in question is later than 244 and returns false otherwise.
// (systemd v244-rc1 is the earliest version to allow restarts for
// oneshot services).
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

// showHelp prints the help documentations through a pager.
func showHelp() {
    doc := `                                     bat(1)
NAME
    bat -- battery management utility for Linux laptops 

SYNOPSIS
    bat [-chprst] [-t num]

DESCRIPTION
    The following options are available:

    -c, --capacity  
        Print the current battery level.
    -h, --help      
        Print this help document.
    -p, --persist   
        Persist the current threshold between restarts.
    -r, --reset    
        Undoes the persistence setting of the charging threshold between 
        restarts.
    -s, --status
        Print the charging status.
    -t, --threshold num
        Print the current charging threshold limit.
        If num is specified, which should be a value between 1 and 100, this
        will set a new charging threshold limit.
    -v, --version
        Display version information and exit.

EXAMPLES
    - Print the current battery charging threshold.

        $ bat --threshold
    
    - Set a new charging threshold, say of 80%. This usually requires superuser
      permissions.

        $ sudo bat --threshold 80
    
    - Persist the current charging threshold setting between restarts. This also
      usually requires superuser permissions.

        $ sudo bat --persist

SUPPORT
    Report any issues on https://github.com/leveson/bat/issues.

REFERENCE
    https://wiki.archlinux.org/title/Laptop/ASUS#Battery_charge_threshold

                                  10 JUNE 2021
`
    cmd := exec.Command("less", "--IGNORE-CASE", "--quit-if-one-screen")
    cmd.Stdin = strings.NewReader(doc)
    cmd.Stdout = os.Stdout
    err := cmd.Run()
    if err != nil {
        log.Fatal(err)
    }
}

// persist persists the prevailing battery charging threshold level
// between restarts and sleep by creating or updating the systemd
// services `bat-boot.service` and `bat-sleep.service`.
func persist() {
    type Service struct {
        Event     string
        Target    string
        Threshold string
    }
    threshold := fileContents("/sys/class/power_supply/BAT?/" +
        "charge_control_end_threshold")
    units := []Service{
        {"boot", "multi-user", threshold},
        {"sleep", "suspend", threshold},
    }
    unit := `
[Unit]
Description=Persist the battery charging threshold between {{ .Event }} 
After={{ .Target }}.target
StartLimitBurst=0

[Service]
Type=oneshot
Restart=on-failure
ExecStart=/usr/bin/bash -c 'echo {{ .Threshold }} > /sys/class/power_supply/BAT?/charge_control_end_threshold'

[Install]
WantedBy={{ .Target }}.target
    `
    tmpl, err := template.New("unit").Parse(unit)
    if err != nil {
        log.Fatal(err)
    }
    for _, service := range units {
        f, err := os.Create(
            fmt.Sprintf("/etc/systemd/system/bat-%s.service", service.Event))
        if err != nil {
            if strings.HasSuffix(err.Error(), ": permission denied") {
                fmt.Println("This command requires sudo permissions.")
                os.Exit(1)
            }
            log.Fatal(err)
        }
        defer f.Close()
        err = tmpl.Execute(f, service)
        if err != nil {
            log.Fatal(err)
        }
        cmd := exec.Command("systemctl", "enable",
            fmt.Sprintf("bat-%s.service", service.Event))
        err = cmd.Run()
        if err != nil {
            log.Fatal(err)
        }
    }
    fmt.Printf("Persistence of the current charging threshold (%s%%) " +
        "enabled.\n", threshold)
}

// reset disables the systemd services that persist the charging
// threshold between restarts and sleep.
func reset() {
    for _, service := range []string{"boot", "sleep"} {
        err := os.Remove(
            fmt.Sprintf("/etc/systemd/system/bat-%s.service", service))
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
        cmd := exec.Command("systemctl", "disable",
            fmt.Sprintf("bat-%s.service", service))
        var stdErr bytes.Buffer
        cmd.Stderr = &stdErr
        err = cmd.Run()
        if err != nil {
            msg := strings.TrimSpace(stdErr.String())
            if !strings.HasSuffix(msg, fmt.Sprintf(" file bat-%s.service " +
                "does not exist.", service)) {
                log.Fatal(err)
            }
        }
    }
}

// setThreshold sets the charging threshold by writing to the
// `charge_control_end_threshold` variable after gaining superuser
// permissions and prints a message to the terminal about the status of
// the operation.
func setThreshold(threshold int) {
    bat, err := filepath.Glob("/sys/class/power_supply/BAT?/" +
        "charge_control_end_threshold")
    if err != nil {
        log.Fatal(err)
    }
    if len(bat) == 0 {
        fmt.Println("This program is most likely not compatible with your " +
            "system. See\nhttps://github.com/leveson/bat#disclaimer.")
        os.Exit(1)
    }
    f, err := os.Create(bat[0])
    if err != nil {
        if strings.HasSuffix(err.Error(), ": permission denied") {
            fmt.Println("This command requires sudo permissions.")
            os.Exit(1)
        }
        log.Fatal(err)
    }
    defer f.Close()
    f.WriteString(fmt.Sprint(threshold))
    fmt.Printf("\rCharging threshold set to %d%%.\nUse `sudo bat --persist` " +
        "to persist this setting between restarts.\n", threshold)
}

func main() {
    n := len(os.Args)
    if n == 1 {
        showHelp()
        os.Exit(0)
    }
    switch os.Args[1] {
    case "-c", "--capacity":
        fmt.Println(fileContents("/sys/class/power_supply/BAT?/capacity"))
    case "-h", "--help":
        showHelp()
    case "-p", "--persist":
        if !hasRequiredSystemdVer() {
            fmt.Println("Requires systemd version 244-rc1 or later.")
            os.Exit(1)
        }
        persist()
    case "-r", "--reset":
        reset()
    case "-s", "--status":
        fmt.Println(fileContents("/sys/class/power_supply/BAT?/status"))
    case "-t", "--threshold":
        if !hasRequiredKernelVer() {
            fmt.Println("Requires Linux kernel version 5.4 or later to set " +
                "the charging threshold.")
            os.Exit(1)
        }
        switch {
        case n > 3:
            fmt.Println("Expects a single argument.")
            os.Exit(1)
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
            fmt.Println(fileContents("/sys/class/power_supply/BAT?/" +
                "charge_control_end_threshold"))
        }
    case "-v", "--version":
        fmt.Println("bat 0.6")
    default:
        fmt.Printf("There is no %s option. Use `bat --help` to see a list of " +
            "available options.\n", os.Args[1])
        os.Exit(1)
    }
}
