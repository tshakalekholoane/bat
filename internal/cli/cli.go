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
	// quit is the function that sets the exit code.
	quit func(code int)
}

// page filters the string doc through the less pager.
func (a *app) page(doc string) {
	cmd := exec.Command(
		a.pager,
		"--no-init",
		"--quit-if-one-screen",
		"--IGNORE-CASE",
		"--RAW-CONTROL-CHARS",
	)
	cmd.Stdin = strings.NewReader(doc)
	cmd.Stdout = a.console.out
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(a.console.err, "cli: fatal error: %v\n", err)
		a.console.quit(failure)
		return
	}

	a.console.quit(success)
}

// errorf formats according to a format specifier, prints to standard
// error, and exits with an error code 1.
func (c *console) errorf(format string, a ...any) {
	fmt.Fprintf(c.err, format, a...)
	c.quit(failure)
}

// errorln formats using the default format for its operands, appends a
// new line, writes to standard error, and exits with error code 1.
func (c *console) errorln(a ...any) {
	c.errorf("%v\n", a...)
}

// writef formats according to a format specifier, prints to standard
// input.
func (c *console) writef(format string, a ...any) {
	fmt.Fprintf(c.out, format, a...)
}

// writeln formats using the default format for its operands, appends a
// new line, and writes to standard input.
func (c *console) writeln(a ...any) {
	c.writef("%v\n", a...)
}

// app represents this application and its dependencies.
type app struct {
	console *console
	// pager is the path of pager pager.
	pager string
	// read is the function used to read the values of the battery
	// variable.
	read func(variable.Variable) (string, error)
}

// show prints the value of a /sys/class/power_supply/BAT?/ variable.
func (a *app) show(v variable.Variable) {
	val, err := a.read(v)
	if err != nil {
		if errors.Is(err, variable.ErrNotFound) {
			a.console.errorln(incompat)
			return
		}
		log.Fatalln(err)
	}
	a.console.writeln(val)
}

// Run executes the application.
func Run() {
	app := &app{
		console: &console{
			err:  os.Stderr,
			out:  os.Stdout,
			quit: os.Exit,
		},
		pager: "less",
		read:  variable.Val,
	}

	if len(os.Args) == 1 {
		app.page(help)
	}

	switch os.Args[1] {
	// Generic program information.
	case "-h", "--help":
		app.page(help)
	case "-v", "--version":
		app.page(info(tag, time.Now()))
	// Subcommands.
	case "capacity":
		app.show(variable.Capacity)
	case "persist":
		if err := services.Write(); err != nil {
			switch {
			case errors.Is(err, services.ErrBashNotFound):
				app.console.errorln("Could not find Bash on your system.")
			case errors.Is(err, services.ErrIncompatSystemd):
				app.console.errorln("Requires systemd version 244-rc1 or later.")
			case errors.Is(err, variable.ErrNotFound):
				app.console.errorln(incompat)
			case errors.Is(err, errPermissionDenied):
				app.console.errorln(noPermission)
			default:
				log.Fatalln(err)
			}
		}
		app.console.writeln("Persistence of the current charging threshold enabled.")
	case "reset":
		if err := services.Delete(); err != nil {
			if errors.Is(err, errPermissionDenied) {
				app.console.errorln(noPermission)
			}
			log.Fatal(err)
		}
		app.console.writeln("Charging threshold persistence reset.")
	case "status":
		app.show(variable.Status)
	case "threshold":
		switch {
		case len(os.Args) > 3:
			app.console.errorln("Expects a single argument.")
		case len(os.Args) == 3:
			t, err := strconv.Atoi(os.Args[2])
			if err != nil {
				if errors.Is(err, strconv.ErrSyntax) {
					app.console.errorln("Argument should be an integer.")
				}
				log.Fatal(err)
			}

			if !threshold.IsValid(t) {
				app.console.errorln("Number should be between 1 and 100.")
			}

			if err := threshold.Set(t); err != nil {
				switch {
				case errors.Is(err, threshold.ErrIncompatKernel):
					app.console.errorln("Requires Linux kernel version 5.4 or later.")
				case errors.Is(err, variable.ErrNotFound):
					app.console.errorln(incompat)
				case errors.Is(err, errPermissionDenied):
					app.console.errorln(noPermission)
				default:
					log.Fatal(err)
				}
			}
			app.console.writeln("Charging threshold set.\nRun `sudo bat persist` to persist the setting between restarts.")
		default:
			app.show(variable.Threshold)
		}
	default:
		app.console.errorf(
			"There is no %s option. Run `bat --help` to see a list of available options.\n",
			os.Args[1],
		)
	}
}
