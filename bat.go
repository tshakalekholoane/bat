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

var help string = `
                                      bat
									  
NAME
	bat - battery management utility for Linux laptops 

SYNOPSIS
	bat [OPTION]
	
DESCRIPTION
    -c, --capacity,     print current battery level
    -h, --help,         print this help document
    -t, --threshold,    print the current charging threshold limit
                        append a value between 1 and 100 to set a new threshold
    -s, --status        print charging status

REFERENCE
	https://wiki.archlinux.org/index.php/Laptop/ASUS#Battery_charge_threshold

                                13 JANUARY 2021
`

func cat(file string) {
	cmd := exec.Command("cat", file)
	out, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(string(out))
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

func setThreshold(t int) {
	st := fmt.Sprintf("echo %d > "+
		"/sys/class/power_supply/BAT0/charge_control_end_threshold", t)
	cmd := exec.Command("su", "-c", st)
	fmt.Print("Root password: ")
	cmd.Stdin = os.Stdin
	err := cmd.Run()
	fmt.Println()
	if err != nil {
		fmt.Println("Authentication failure.")
		os.Exit(1)
	}
}

func main() {
	nArgs := len(os.Args)

	if nArgs == 1 {
		page(help)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "-c", "--capacity":
		cat("/sys/class/power_supply/BAT0/capacity")
	case "-h", "--help":
		page(help)
	case "-t", "--threshold":
		switch {
		case nArgs > 3:
			fmt.Print("Expects single argument.\n")
		case nArgs == 3:
			t, err := strconv.Atoi(os.Args[2])
			if err != nil {
				if errors.Is(err, strconv.ErrSyntax) {
					fmt.Print("Argument should be an integer.\n")
					os.Exit(1)
				} else {
					log.Fatal(err)
				}
			}
			if t < 1 || t > 100 {
				fmt.Print("Number should be between 1 and 100.\n")
				os.Exit(1)
			}
			setThreshold(t)
			fmt.Printf("Charging threshold set to %s.\n", os.Args[2])
		default:
			cat("/sys/class/power_supply/BAT0/charge_control_end_threshold")
			os.Exit(0)
		}
	case "-s", "--status":
		cat("/sys/class/power_supply/BAT0/status")
	default:
		fmt.Printf("There is no %s option. Use bat --help to see a list of"+
			"available options.\n", os.Args[1])
	}
}
