// Package cli handles the command line user interface for bat.
package cli

import (
	"bytes"
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
	"text/template"
	"time"

	"tshaka.co/bat/internal/services"
	"tshaka.co/bat/internal/threshold"
	"tshaka.co/bat/internal/variable"
)

// Unix status codes.
const (
	success = iota
	failure
)

// Common error messages.
const (
	incompat = `This program is most likely not compatible with your system. See
https://github.com/tshakalekholoane/bat#disclaimer for details.`
	noPermission = "Permission denied. Try running this command using sudo."
)

// errPermissionDenied indicates that the user has insufficient
// permissions to perform an action.
var errPermissionDenied = syscall.EACCES

// tag is the version information evaluated at compile time.
var tag string

var (
	//go:embed help.txt
	help string
	//go:embed version.tmpl
	version string
)

// info returns the version information as a string.
func info(tag string, now time.Time) string {
	buf := new(bytes.Buffer)
	tmpl := template.Must(template.New("version").Parse(version))
	tmpl.Execute(buf, struct {
		Tag  string
		Year int
	}{
		tag,
		time.Now().Year(),
	})
	return buf.String()
}

// console represents a text terminal user interface.
type console struct {
	// err represents standard error.
	err io.Writer
	// out represents standard output.
	out io.Writer
	// pager is the path of pager pager.
	pager string
	// quit is the function that sets the exit code.
	quit func(code int)
}

// page filters the string doc through the less pager.
func (c *console) page(doc string) {
	cmd := exec.Command(
		c.pager,
		"--no-init",
		"--quit-if-one-screen",
		"--IGNORE-CASE",
		"--RAW-CONTROL-CHARS",
	)
	cmd.Stdin = strings.NewReader(doc)
	cmd.Stdout = c.out
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(c.err, "cli: fatal error: %v\n", err)
		c.quit(failure)
		return
	}

	c.quit(success)
}

// errorf formats according to a format specifier, prints to standard
// error, and exits with an error code 1.
func errorf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format, a...)
	os.Exit(1)
}

// errorln formats using the default format for its operands, appends a
// new line, writes to standard error, and exits with error code 1.
func errorln(a ...any) {
	errorf("%v\n", a...)
}

// writef formats according to a format specifier, prints to standard
// input.
func writef(format string, a ...any) {
	fmt.Fprintf(os.Stdout, format, a...)
}

// writeln formats using the default format for its operands, appends a
// new line, and writes to standard input.
func writeln(a ...any) {
	writef("%v\n", a...)
}

// show prints the value of a /sys/class/power_supply/BAT?/ variable.
func show(v variable.Variable) {
	c, err := variable.Val(v)
	if err != nil {
		if errors.Is(err, variable.ErrNotFound) {
			errorln(incompat)
		}
		log.Fatalln(err)
	}
	writeln(c)
}

// Run executes the application.
func Run() {
	cons := console{
		err:   os.Stderr,
		out:   os.Stdout,
		pager: "less",
		quit:  os.Exit,
	}

	if len(os.Args) == 1 {
		cons.page(help)
	}

	switch os.Args[1] {
	// Generic program information.
	case "-h", "--help":
		cons.page(help)
	case "-v", "--version":
		cons.page(info(tag, time.Now()))
	// Subcommands.
	case "capacity":
		show(variable.Capacity)
	case "persist":
		if err := services.Write(); err != nil {
			switch {
			case errors.Is(err, services.ErrBashNotFound):
				errorln("Could not find Bash on your system.")
			case errors.Is(err, services.ErrIncompatSystemd):
				errorln("Requires systemd version 244-rc1 or later.")
			case errors.Is(err, variable.ErrNotFound):
				errorln(incompat)
			case errors.Is(err, errPermissionDenied):
				errorln(noPermission)
			default:
				log.Fatalln(err)
			}
		}
		writeln("Persistence of the current charging threshold enabled.")
	case "reset":
		if err := services.Delete(); err != nil {
			if errors.Is(err, errPermissionDenied) {
				errorln(noPermission)
			}
			log.Fatal(err)
		}
		writeln("Charging threshold persistence reset.")
	case "status":
		show(variable.Status)
	case "threshold":
		switch {
		case len(os.Args) > 3:
			errorln("Expects a single argument.")
		case len(os.Args) == 3:
			t, err := strconv.Atoi(os.Args[2])
			if err != nil {
				if errors.Is(err, strconv.ErrSyntax) {
					errorln("Argument should be an integer.")
				}
				log.Fatal(err)
			}

			if !threshold.IsValid(t) {
				errorln("Number should be between 1 and 100.")
			}

			if err := threshold.Set(t); err != nil {
				switch {
				case errors.Is(err, threshold.ErrIncompatKernel):
					errorln("Requires Linux kernel version 5.4 or later.")
				case errors.Is(err, variable.ErrNotFound):
					errorln(incompat)
				case errors.Is(err, errPermissionDenied):
					errorln(noPermission)
				default:
					log.Fatal(err)
				}
			}
			writeln("Charging threshold set.\nRun `sudo bat persist` to persist the setting between restarts.")
		default:
			show(variable.Threshold)
		}
	default:
		errorf(
			"There is no %s option. Run `bat --help` to see a list of available options.\n",
			os.Args[1],
		)
	}
}
