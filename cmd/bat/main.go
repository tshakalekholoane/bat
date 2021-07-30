package main

import (
    "errors"
    "fmt"
    "log"
    "os"
    "strconv"
    "strings"

    "github.com/leveson/bat/internal/docs"
    "github.com/leveson/bat/internal/io"
    "github.com/leveson/bat/internal/persist"
    "github.com/leveson/bat/internal/threshold"
)

// printFile is a wrapper around `io.FileContents` to simplify printing
// the values of some (battery) virtual files.
func printFile(vf string) {
    s, err := io.FileContents(vf)
    if err != nil {
        if err.Error() == "virtual file not found" {
            fmt.Println(
                "This program is most likely not compatible with your " +
                "system. See\nhttps://github.com/leveson/bat#disclaimer.")
            os.Exit(1)
        }
        log.Fatal(err)
    }
    fmt.Println(s)
}

func main() {
    if len(os.Args) == 1 {
        docs.Help()
        os.Exit(0)
    }
    switch os.Args[1] {
    case "-c", "--capacity":
        printFile("capacity")
    case "-h", "--help":
        err := docs.Help()
        if err != nil {
            log.Fatal(err)
        }
    case "-p", "--persist":
        err := persist.WriteServices()
        if err != nil {
            switch {
            case err.Error() == "bash not found":
                fmt.Println("Requires Bash to persist the charging threshold.")
                os.Exit(1)
            case err.Error() == "incompatible systemd version":
                fmt.Println("Requires systemd version 244-rc1 or later.")
                os.Exit(1)
            case err.Error() == "virtual file not found": 
                fmt.Println(
                    "This program is most likely not compatible with your " +
                    "system. See\nhttps://github.com/leveson/bat#disclaimer.")
                os.Exit(1)
            case strings.HasSuffix(err.Error(), "permission denied"):
                fmt.Println("This command requires sudo permissions.")
                os.Exit(1)
            default:
                log.Fatal(err)
            }
        }
        fmt.Println("Persistence of the current charging threshold enabled.")
    case "-r", "--reset":
        err := persist.RemoveServices()
        if err != nil {
            if strings.HasSuffix(err.Error(), "permission denied") {
                fmt.Println("This command requires sudo permissions.")
                os.Exit(1)
            }
            log.Fatal(err)
        }
        fmt.Println("Charging threshold persistence reset.")
    case "-s", "--status":
        printFile("status")
    case "-t", "--threshold":
        switch {
        case len(os.Args) > 3:
            fmt.Println("Expects a single argument.")
            os.Exit(1)
        case len(os.Args) == 3:
            t, err := strconv.Atoi(os.Args[2])
            if err != nil {
                if errors.Is(err, strconv.ErrSyntax) {
                    fmt.Println("Argument should be an integer.")
                    os.Exit(1)
                }
                log.Fatal(err)
            }
            if t < 1 || t > 100 {
                fmt.Println("Number should be between 1 and 100.")
                os.Exit(1)
            }
            err = threshold.Write(t)
            if err != nil {
                switch {
                case err.Error() == "incompatible kernel version":
                    fmt.Println("Requires Linux kernel version 5.4 or later.")
                    os.Exit(1)
                case err.Error() == "virtual file not found":
                    fmt.Println(
                        "This program is most likely not compatible with " +
                        "your system. See\n" +
                        "https://github.com/leveson/bat#disclaimer.")
                    os.Exit(1)
                case strings.HasSuffix(err.Error(), "permission denied"):
                    fmt.Println("This command requires sudo permissions.")
                    os.Exit(1)
                default:
                    log.Fatal(err)
                }
            }
            fmt.Println(
                "Charging threshold set.\nUse `sudo bat --persist` to " +
                "persist the setting between restarts.")
        default:
            printFile("charge_control_end_threshold")
        }
    case "-v", "--version":
        err := docs.VersionInfo()
        if err != nil {
            log.Fatal(err)
        }
    default:
        fmt.Printf(
            "There is no %s option. Use `bat --help` to see a list of " +
                "available options.\n",
            os.Args[1])
        os.Exit(1)
    }
}
