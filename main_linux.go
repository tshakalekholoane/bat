// Binary bat is a battery management utility for Linux laptops.
package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	rtdebug "runtime/debug"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"golang.org/x/sys/unix"
)

type Service struct {
	Event, Path, Shell string
	Threshold          int
}

type Target struct {
	Unit string `json:"unit"`
}

const threshold = "charge_control_end_threshold"

var (
	tag string

	events = [...]string{
		"hibernate",
		"hybrid-sleep",
		"multi-user",
		"suspend",
		"suspend-then-hibernate",
	}

	services = filepath.Join("/", "etc", "systemd", "system")

	//go:embed bat.service
	unit string

	//go:embed help.txt
	usage string
)

type battery struct {
	root string
}

func (b *battery) has(variable string) (bool, error) {
	_, err := os.Stat(b.path(variable))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (b *battery) path(variable string) string {
	return filepath.Join(b.root, variable)
}

func (b *battery) read(variable string) (string, error) {
	contents, err := os.ReadFile(b.path(variable))
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(contents)), nil
}

func (b *battery) write(variable string, contents []byte) error {
	return os.WriteFile(b.path(variable), contents, 0o644)
}

func main() {
	// Default flag.Usage is overridden below.
	const ignore = ""
	var (
		d, debug   = flag.Bool("d", false, ignore), flag.Bool("debug", false, ignore)
		h, help    = flag.Bool("h", false, ignore), flag.Bool("help", false, ignore)
		v, version = flag.Bool("v", false, ignore), flag.Bool("version", false, ignore)
	)
	flag.Usage = func() {
		fmt.Print(usage)
	}
	flag.Parse()

	if *h || *help {
		flag.Usage()
		return
	}

	if *v || *version {
		fmt.Printf("bat %s\nCopyright (c) 2021 Tshaka Lekholoane.\nMIT Licence.\n", tag)
		return
	}

	defer func() {
		if err := recover(); err != nil {
			var message string
			if *d || *debug {
				message = fmt.Sprintf("%s\n\n%s", err, string(rtdebug.Stack()))
			} else {
				message = "A fatal error occurred. Please rerun the command with the `--debug` flag\n" +
					"enabled, and file an issue with the resulting output to the following address:\n" +
					"https://github.com/tshakalekholoane/bat/issues/new."
			}
			fmt.Fprintln(os.Stderr, message)
		}
	}()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	batteries, err := filepath.Glob(filepath.Join("/", "sys", "class", "power_supply", "BAT?"))
	if err != nil {
		panic(err)
	}
	if len(batteries) == 0 {
		fmt.Fprintln(
			os.Stderr,
			"This program is most likely not compatible with your system. See\n"+
				"https://github.com/tshakalekholoane/bat#disclaimer for details.",
		)
		os.Exit(1)
	}
	// Default to using the first battery.
	bat := &battery{root: batteries[0]}

	switch subcommand := flag.Arg(0); subcommand {
	case "capacity", "status":
		v, err := bat.read(subcommand)
		if err != nil {
			panic(err)
		}
		fmt.Println(v)
	case "health":
		// Some devices use charge_* and others energy_* so probe both. The
		// health is computed as x / y where x is the eroded capacity and y
		// is the capacity when the battery was new.
		var (
			err  error
			v, w string
		)
		s, t := "charge_full", "charge_full_design"
		v, err = bat.read(s)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				goto energy
			}
			panic(err)
		}
		w, err = bat.read(t)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			panic(err)
		}
		goto health
	energy:
		// Should have one or the other.
		s, t = "energy_full", "energy_full_design"
		v, err = bat.read(s)
		if err != nil {
			panic(err)
		}
		w, err = bat.read(t)
		if err != nil {
			panic(err)
		}
	health:
		x, err := strconv.Atoi(v)
		if err != nil {
			panic(err)
		}
		y, err := strconv.Atoi(w)
		if err != nil {
			panic(err)
		}
		fmt.Println(x * 100 / y)
	case "persist":
		ok, err := bat.has(threshold)
		if err != nil {
			panic(err)
		}
		if !ok {
			fmt.Fprintln(os.Stderr, "Charging threshold setting not found.")
			os.Exit(1)
		}

		// systemd 244-rc1 is the earliest version to allow restarts for
		// oneshot services.
		output, err := exec.Command("systemctl", "--version").CombinedOutput()
		if err != nil {
			panic(err)
		}
		var revision int
		_, err = fmt.Sscanf(string(output), "systemd %d", &revision)
		if err != nil {
			panic(err)
		}
		if revision < 244 {
			fmt.Fprintln(os.Stderr, "Requires systemd version 243-rc1 or later.")
			os.Exit(1)
		}

		shell, err := exec.LookPath("sh")
		if err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				fmt.Fprintln(os.Stderr, "Could not find `sh` in your `$PATH`.")
				os.Exit(1)
			}
			panic(err)
		}

		v, err := bat.read(threshold)
		if err != nil {
			panic(err)
		}
		current, err := strconv.Atoi(v)
		if err != nil {
			panic(err)
		}

		// Creates services for events with defined targets (targets vary by
		// distribution).
		cmd := exec.Command("systemctl", "list-units", "--type", "target", "--all", "--plain", "--output", "json")
		output, err = cmd.CombinedOutput()
		if err != nil {
			panic(err)
		}
		targets := make([]Target, 0)
		if err = json.Unmarshal(output, &targets); err != nil {
			panic(err)
		}
		available := make([]string, 0)
		for _, target := range targets {
			event := strings.TrimSuffix(target.Unit, ".target")
			if slices.Index(events[:], event) != -1 {
				available = append(available, event)
			}
		}
		tmpl := template.Must(template.New("unit").Parse(unit))
		for _, event := range available {
			service := "bat-" + event + ".service"
			f, err := os.Create(filepath.Join(services, service))
			if err != nil {
				if errors.Is(err, unix.EACCES) {
					fmt.Fprintln(os.Stderr, "Permission denied. Try running this command with `sudo`.")
					os.Exit(1)
				}
				panic(err)
			}
			s := Service{
				Event:     event,
				Path:      bat.path(threshold),
				Shell:     shell,
				Threshold: current,
			}
			if err = tmpl.Execute(f, s); err != nil {
				panic(err)
			}
			if err = exec.Command("systemctl", "enable", service).Run(); err != nil {
				panic(err)
			}
			if err = f.Close(); err != nil {
				panic(err)
			}
		}
		fmt.Println("Persistence of the current charging threshold enabled.")
	case "threshold":
		ok, err := bat.has(threshold)
		if err != nil {
			panic(err)
		}
		if !ok {
			fmt.Fprintln(os.Stderr, "Charging threshold setting not found.")
			os.Exit(1)
		}
		switch flag.NArg() {
		case 1:
			// Get.
			var v string
			v, err = bat.read(threshold)
			if err != nil {
				panic(err)
			}
			fmt.Println(v)
		case 2:
			// Set.
			// The earliest version of the Linux kernel to expose the battery
			// charging threshold is 5.4.
			var utsname unix.Utsname
			if err = unix.Uname(&utsname); err != nil {
				panic(err)
			}
			var maj, min int
			_, err = fmt.Sscanf(string(utsname.Release[:]), "%d.%d", &maj, &min)
			if err != nil {
				panic(err)
			}
			if maj <= 5 && (maj != 5 || min < 4) {
				fmt.Fprintln(os.Stderr, "Requires Linux kernel version 5.4 or later.")
				os.Exit(1)
			}

			setting := flag.Arg(1)
			i, err := strconv.Atoi(setting)
			if err != nil {
				if errors.Is(err, strconv.ErrSyntax) {
					fmt.Fprintln(os.Stderr, "Argument should be an integer.")
					os.Exit(1)
				}
				panic(err)
			}
			if i < 1 || i > 100 {
				fmt.Fprintln(os.Stderr, "Threshold value should be between 1 and 100.")
				os.Exit(1)
			}
			if err := bat.write(threshold, []byte(setting)); err != nil {
				if errors.Is(err, unix.EACCES) {
					fmt.Fprintln(os.Stderr, "Permission denied. Try running this command with `sudo`.")
					os.Exit(1)
				}
				panic(err)
			}
			fmt.Println("Charging threshold set.\n" +
				"Run `sudo bat persist` to persist the setting between restarts.")
		default:
			fmt.Fprintln(os.Stderr, "Invalid number of arguments.")
			flag.Usage()
			os.Exit(1)
		}
	case "reset":
		for _, event := range events {
			service := "bat-" + event + ".service"
			output, err := exec.Command("systemctl", "disable", service).CombinedOutput()
			if err != nil {
				// WORKAROUND: systemd returns the generic exit code 1 for all
				// failures, so triage using a substring search on the output.
				// This method may be unreliable in non-EN locales.
				switch {
				case bytes.Contains(output, []byte("authentication required")):
					fmt.Fprintln(os.Stderr, "Permission denied. Try running this command with `sudo`.")
					os.Exit(1)
				case bytes.Contains(output, []byte("service does not exist")):
					continue
				default:
					panic(string(output))
				}
			}
			err = os.Remove(filepath.Join(services, service))
			if err != nil && !errors.Is(err, unix.ENOENT) {
				if errors.Is(err, unix.EACCES) {
					fmt.Fprintln(os.Stderr, "Permission denied. Try running this command with `sudo`.")
					os.Exit(1)
				}
				panic(err)
			}
		}
		fmt.Println("Charging threshold persistence reset.")
	default:
		fmt.Fprintf(
			os.Stderr,
			"There is no `%s` command. Run `bat --help` to see a list of available commands.\n",
			subcommand,
		)
		os.Exit(1)
	}
}
