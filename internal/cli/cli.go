// Package cli handles the command line user interface for bat.
package cli

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"syscall"

	"tshaka.co/bat/internal/docs"
	"tshaka.co/bat/internal/file"
	"tshaka.co/bat/internal/persist"
	"tshaka.co/bat/internal/threshold"
)

// common error messages
const (
	incompat = "This program is most likely not compatible with your system. " +
		"See\nhttps://github.com/tshakalekholoane/bat#disclaimer for details."
	permissionDenied = "Permission denied. Try running this command using sudo."
)

// errPermissionDenied indicates that the user has insufficient
// permissions to perform an action.
var errPermissionDenied = syscall.EACCES

// context is a helper function that returns an io.Writer and a status
// code the program should exit with after performing an action.
func context(fatal bool) (io.Writer, int) {
	var out io.Writer = os.Stdout
	var code int
	if fatal {
		out = os.Stderr
		code = 1
	}
	return out, code
}

// reportf formats according to a format specifier and writes to either
// standard error or standard output depending on the context.
func reportf(fatal bool, format string, a ...interface{}) {
	out, code := context(fatal)
	fmt.Fprintf(out, format, a...)
	os.Exit(code)
}

// reportln formats using the default formats for its operands, appends
// a new line and, writes to standard output.
func reportln(fatal bool, a ...interface{}) {
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
		docs.Usage()
		os.Exit(0)
	}
	switch os.Args[1] {
	case "-c", "--capacity":
		show("capacity")
	case "-h", "--help":
		err := docs.Usage()
		if err != nil {
			log.Fatalln(err)
		}
		os.Exit(0)
	case "-p", "--persist":
		err := persist.WriteServices()
		if err != nil {
			switch {
			case errors.Is(err, persist.ErrBashNotFound):
				reportln(true, "Cannot find Bash on your system.")
			case errors.Is(err, persist.ErrIncompatSystemd):
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
		err := persist.DeleteServices()
		if err != nil {
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
			if t < 1 || t > 100 {
				reportln(true, "Number should be between 1 and 100.")
			}
			err = threshold.Set(t)
			if err != nil {
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
		err := docs.Version()
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	default:
		reportf(
			true,
			"There is no %s option. Run `bat --help` to see a list of available "+
				"options.\n",
			os.Args[1])
	}
}
