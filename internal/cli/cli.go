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
	"time"

	"golang.org/x/sys/unix"
	"tshaka.dev/x/bat/internal/systemd"
	"tshaka.dev/x/bat/pkg/power"
)

const (
	success = iota
	failure
)

const (
	msgArgNotInt              = "Argument should be an integer."
	msgBashNotFound           = "Could not find Bash on your system."
	msgExpectedSingleArg      = "Expects a single argument."
	msgIncompatible           = "This program is most likely not compatible with your system. See\nhttps://github.com/tshakalekholoane/bat#disclaimer for details."
	msgIncompatibleKernel     = "Requires Linux kernel version 5.4 or later."
	msgIncompatibleSystemd    = "Requires systemd version 243-rc1 or later."
	msgNoOption               = "There is no %s option. Run `bat --help` to see a list of available options.\n"
	msgOutOfRangeThresholdVal = "Number should be between 1 and 100."
	msgPermissionDenied       = "Permission denied. Try running this command using sudo."
	msgPersistenceEnabled     = "Persistence of the current charging threshold enabled."
	msgPersistenceReset       = "Charging threshold persistence reset."
	msgThresholdSet           = "Charging threshold set.\nRun `sudo bat persist` to persist the setting between restarts."
)

// tag is the version information evaluated at compile time.
var tag string

var (
	//go:embed help.txt
	help string
	//go:embed version.tmpl
	version string
)

// resetWriter is the interface that groups the Reset and Write methods
// used to write and remove systemd services.
type resetWriter interface {
	Reset() error
	Write() error
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

// app represents this application and its dependencies.
type app struct {
	console *console
	// pager is the path of the pager.
	pager string
	// get is the function used to read the value of the battery variable.
	get func(power.Variable) (string, error)
	// set is the function used to write the battery charging threshold
	// value.
	set func(power.Variable, string) error
	// systemder is used to write and delete systemd services that persist
	// the charging threshold between restarts.
	systemder resetWriter
}

// errorf formats according to a format specifier, prints to standard
// error, and exits with an error code 1.
func (a *app) errorf(format string, v ...any) {
	fmt.Fprintf(a.console.err, format, v...)
	a.console.quit(failure)
}

// errorln formats using the default format for its operands, appends a
// new line, writes to standard error, and exits with error code 1.
func (a *app) errorln(v ...any) { a.errorf("%v\n", v...) }

// writef formats according to a format specifier, prints to standard
// input.
func (a *app) writef(format string, v ...any) {
	fmt.Fprintf(a.console.out, format, v...)
}

// writeln formats using the default format for its operands, appends a
// new line, and writes to standard input.
func (a *app) writeln(v ...any) { a.writef("%v\n", v...) }

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

// show prints the value of the given /sys/class/power_supply/BAT?/
// variable.
func (a *app) show(v power.Variable) {
	val, err := a.get(v)
	if err != nil {
		if errors.Is(err, power.ErrNotFound) {
			a.errorln(msgIncompatible)
			return
		}
		log.Fatalln(err)
	}
	a.writeln(val)
}

func (a *app) help() { a.page(help) }

func (a *app) version() {
	buf := new(bytes.Buffer)
	buf.Grow(96 /* maximum buffer length when branch is dirty is ≈ 84 */)
	fmt.Fprintf(buf, version, tag, time.Now().Year())
	a.page(buf.String())
}

func (a *app) capacity() { a.show(power.Capacity) }

func (a *app) persist() {
	if err := a.systemder.Write(); err != nil {
		// XXX: Can't switch over wrapped errors.
		switch {
		case errors.Is(err, systemd.ErrBashNotFound):
			a.errorln(msgBashNotFound)
			return
		case errors.Is(err, systemd.ErrIncompatSystemd):
			a.errorln(msgIncompatibleSystemd)
			return
		case errors.Is(err, power.ErrNotFound):
			a.errorln(msgIncompatible)
			return
		case errors.Is(err, syscall.EACCES):
			a.errorln(msgPermissionDenied)
			return
		default:
			log.Fatalln(err)
		}
	}
	a.writeln(msgPersistenceEnabled)
}

func (a *app) reset() {
	if err := a.systemder.Reset(); err != nil {
		if errors.Is(err, syscall.EACCES) {
			a.errorln(msgPermissionDenied)
			return
		}
		log.Fatal(err)
	}
	a.writeln(msgPersistenceReset)
}

func (a *app) status() { a.show(power.Status) }

// valid returns true if threshold is in the range 1..=100.
func valid(threshold int) bool { return threshold >= 1 && threshold <= 100 }

// kernel returns the Linux kernel version as a string and an error
// otherwise.
func kernel() (string, error) {
	var name unix.Utsname
	if err := unix.Uname(&name); err != nil {
		return "", err
	}
	return string(name.Release[:]), nil
}

// isRequiredKernel returns true if the string ver represents a
// semantic version later than 5.4 and false otherwise (this is the
// earliest version of the Linux kernel to expose the battery charging
// threshold variable). It also returns an error if it failed parse the
// string.
func requiredKernel(ver string) (bool, error) {
	var maj, min int
	_, err := fmt.Sscanf(ver, "%d.%d", &maj, &min)
	if err != nil {
		return false, err
	}
	if maj > 5 /* 🤷 */ || (maj == 5 && min >= 4) {
		return true, nil
	}
	return false, nil
}

func (a *app) threshold(args []string) {
	switch {
	case len(args) > 3:
		a.errorln(msgExpectedSingleArg)
		return
	case len(args) == 3:
		val := args[2]
		t, err := strconv.Atoi(val)
		if err != nil {
			if errors.Is(err, strconv.ErrSyntax) {
				a.errorln(msgArgNotInt)
				return
			}
			log.Fatal(err)
		}

		if !valid(t) {
			a.errorln(msgOutOfRangeThresholdVal)
			return
		}

		ver, err := kernel()
		if err != nil {
			log.Fatal(err)
			return
		}

		ok, err := requiredKernel(ver)
		if err != nil {
			log.Fatal(err)
			return
		}
		if !ok {
			a.errorln(msgIncompatibleKernel)
			return
		}

		if err := a.set(power.Threshold, strings.TrimSpace(val)); err != nil {
			switch {
			case errors.Is(err, power.ErrNotFound):
				a.errorln(msgIncompatible)
				return
			case errors.Is(err, syscall.EACCES):
				a.errorln(msgPermissionDenied)
				return
			default:
				log.Fatal(err)
			}
		}
		a.writeln(msgThresholdSet)
	default:
		a.show(power.Threshold)
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
		pager:     "less",
		get:       power.Get,
		set:       power.Set,
		systemder: systemd.New(),
	}

	if len(os.Args) == 1 {
		app.help()
	}

	switch command := os.Args[1]; command {
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
		app.errorf(msgNoOption, command)
	}
}
