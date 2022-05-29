// Package cli handles the command line user interface for bat.
package cli

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"tshaka.co/bat/internal/file"
	"tshaka.co/bat/internal/services"
	"tshaka.co/bat/internal/threshold"
)

// Common error messages.
const (
	incompat = "This program is most likely not compatible with your system. " +
		"See\nhttps://github.com/tshakalekholoane/bat#disclaimer for details."
	permissionDenied = "Permission denied. Try running this command using sudo."
)

// Documentation.
var (
	//go:embed help.txt
	help string
	//go:embed version.txt
	version string
)

// errPermissionDenied indicates that the user has insufficient
// permissions to perform an action.
var errPermissionDenied = syscall.EACCES

// context is a helper function that returns an io.Writer and exit code,
// os.Stderr and 1 if fatal is true and os.Stdout and 0 otherwise.
func context(fatal bool) (out io.Writer, code int) {
	if fatal {
		out = os.Stderr
		code = 1
	} else {
		out = os.Stdout
	}
	return
}

// page filters the string doc through the less pager.
func page(doc string) {
	cmd := exec.Command(
		"less",
		"--no-init",
		"--quit-if-one-screen",
		"--IGNORE-CASE",
		"--RAW-CONTROL-CHARS",
	)
	cmd.Stdin = strings.NewReader(doc)
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		log.Fatalln(err)
	}
	os.Exit(0)
}

// reportf formats a according to a format specifier and prints it to
// standard error if fatal is true or to standard output otherwise.
func reportf(fatal bool, format string, a ...any) {
	out, code := context(fatal)
	fmt.Fprintf(out, format, a...)
	os.Exit(code)
}

// reportln formats a using the default formats for its operands,
// appends a new line and, writes to standard error if fatal is true or
// standard output otherwise.
func reportln(fatal bool, a ...any) {
	reportf(fatal, "%v\n", a...)
}

// show prints the value of a /sys/class/power_supply/BAT?/ variable.
func show(v string) {
	c, err := file.Contents(v)
	if err != nil {
		if errors.Is(err, file.ErrNotFound) {
			reportln(true, incompat)
		}
		log.Fatalln(err)
	}
	reportln(false, strings.TrimSpace(string(c)))
}

// Run executes the application.
func Run() {
	if len(os.Args) == 1 {
		page(help)
	}

	switch os.Args[1] {
	case "-c", "--capacity":
		show("capacity")
	case "-h", "--help":
		page(help)
	case "-p", "--persist":
		if err := services.Write(); err != nil {
			switch {
			case errors.Is(err, services.ErrBashNotFound):
				reportln(true, "Could not find Bash on your system.")
			case errors.Is(err, services.ErrIncompatSystemd):
				reportln(true, "Requires systemd version 244-rc1 or later.")
			case errors.Is(err, file.ErrNotFound):
				reportln(true, incompat)
			case errors.Is(err, errPermissionDenied):
				reportln(true, permissionDenied)
			default:
				log.Fatalln(err)
			}
		}
		reportln(false, "Persistence of the current charging threshold enabled.")
	case "-r", "--reset":
		if err := services.Delete(); err != nil {
			if errors.Is(err, errPermissionDenied) {
				reportln(true, permissionDenied)
			}
			log.Fatal(err)
		}
		reportln(false, "Charging threshold persistence reset.")
	case "-s", "--status":
		show("status")
	case "-t", "--threshold":
		switch {
		case len(os.Args) > 3:
			reportln(true, "Expects a single argument.")
		case len(os.Args) == 3:
			t, err := strconv.Atoi(os.Args[2])
			if err != nil {
				if errors.Is(err, strconv.ErrSyntax) {
					reportln(true, "Argument should be an integer.")
				}
				log.Fatal(err)
			}
			if !threshold.IsValid(t) {
				reportln(true, "Number should be between 1 and 100.")
			}
			if err := threshold.Set(t); err != nil {
				switch {
				case errors.Is(err, threshold.ErrIncompatKernel):
					reportln(true, "Requires Linux kernel version 5.4 or later.")
				case errors.Is(err, file.ErrNotFound):
					reportln(true, incompat)
				case errors.Is(err, errPermissionDenied):
					reportln(true, permissionDenied)
				default:
					log.Fatal(err)
				}
			}
			reportln(
				false,
				"Charging threshold set.\nUse `sudo bat --persist` to persist the "+
					"setting between restarts.")
		default:
			show("charge_control_end_threshold")
		}
	case "-v", "--version":
		page(version)
	default:
		reportf(
			true,
			"There is no %s option. Run `bat --help` to see a list of available "+
				"options.\n",
			os.Args[1])
	}
}
