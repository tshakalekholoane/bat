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

const (
	success = iota
	failure
)

const (
	bashNotFound = "Could not find Bash on your system."
	incompatible = `This program is most likely not compatible with your system. See
https://github.com/tshakalekholoane/bat#disclaimer for details.`
	incompatibleSystemd = "Requires systemd version 243-rc1 or later."
	permissionDenied    = "Permission denied. Try running this command using sudo."
	persistenceEnabled  = "Persistence of the current charging threshold enabled."
	persistenceReset    = "Charging threshold persistence reset."
)

// tag is the version information evaluated at compile time.
var tag string

var (
	//go:embed help.txt
	help string
	//go:embed version.tmpl
	version string
)

// console represents a text terminal user interface.
type console struct {
	// err represents standard error.
	err io.Writer
	// out represents standard output.
	out io.Writer
	// quit is the function that sets the exit code.
	quit func(code int)
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
	// service is used to write and delete systemd services that persist
	// the charging threshold between restarts.
	service services.Servicer
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
		log.Fatal(err)
	}
	a.console.quit(success)
}

// show prints the value of a /sys/class/power_supply/BAT?/ variable.
func (a *app) show(v variable.Variable) {
	val, err := a.read(v)
	if err != nil {
		if errors.Is(err, variable.ErrNotFound) {
			a.console.errorln(incompatible)
			return
		}
		log.Fatalln(err)
	}
	a.console.writeln(val)
}

func (a *app) help() {
	a.page(help)
}

func (a *app) version() {
	buf := new(bytes.Buffer)
	tmpl := template.Must(template.New("version").Parse(version))
	tmpl.Execute(buf, struct {
		Tag  string
		Year int
	}{
		tag,
		time.Now().Year(),
	})
	a.page(buf.String())
}

func (a *app) capacity() {
	a.show(variable.Capacity)
}

func (a *app) persist() {
	if err := a.service.Write(); err != nil {
		switch {
		case errors.Is(err, services.ErrBashNotFound):
			a.console.errorln(bashNotFound)
		case errors.Is(err, services.ErrIncompatSystemd):
			a.console.errorln(incompatibleSystemd)
		case errors.Is(err, variable.ErrNotFound):
			a.console.errorln(incompatible)
		case errors.Is(err, syscall.EACCES):
			a.console.errorln(permissionDenied)
		default:
			log.Fatalln(err)
		}
	}
	a.console.writeln(persistenceEnabled)
}

func (a *app) reset() {
	if err := a.service.Delete(); err != nil {
		if errors.Is(err, syscall.EACCES) {
			a.console.errorln(permissionDenied)
			return
		}
		log.Fatal(err)
	}
	a.console.writeln(persistenceReset)
}

func (a *app) status() {
	a.show(variable.Status)
}

func (a *app) threshold(args []string) {
	switch {
	case len(args) > 3:
		a.console.errorln("Expects a single argument.")
	case len(args) == 3:
		t, err := strconv.Atoi(args[2])
		if err != nil {
			if errors.Is(err, strconv.ErrSyntax) {
				a.console.errorln("Argument should be an integer.")
			}
			log.Fatal(err)
		}

		if !threshold.IsValid(t) {
			a.console.errorln("Number should be between 1 and 100.")
		}

		if err := threshold.Set(t); err != nil {
			switch {
			case errors.Is(err, threshold.ErrIncompatKernel):
				a.console.errorln("Requires Linux kernel version 5.4 or later.")
			case errors.Is(err, variable.ErrNotFound):
				a.console.errorln(incompatible)
			case errors.Is(err, syscall.EACCES):
				a.console.errorln(permissionDenied)
			default:
				log.Fatal(err)
			}
		}
		a.console.writeln("Charging threshold set.\nRun `sudo bat persist` to persist the setting between restarts.")
	default:
		a.show(variable.Threshold)
	}
}

// Run executes the application.
func Run() {
	app := &app{
		console: &console{
			err:  os.Stderr,
			out:  os.Stdout,
			quit: os.Exit,
		},
		pager:   "less",
		read:    variable.Get,
		service: &services.Service{},
	}

	if len(os.Args) == 1 {
		app.help()
	}

	switch os.Args[1] {
	// Generic program information.
	case "-h", "--help":
		app.help()
	case "-v", "--version":
		app.version()
	// Subcommands.
	case "capacity":
		app.capacity()
	case "persist":
		app.persist()
	case "reset":
		app.reset()
	case "status":
		app.status()
	case "threshold":
		app.threshold(os.Args)
	default:
		const noOpt = "There is no %s option. Run `bat --help` to see a list of available options.\n"
		app.console.errorf(noOpt, os.Args[1])
	}
}
