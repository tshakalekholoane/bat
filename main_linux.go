// Binary bat is a battery management utility for Linux laptops.
package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"

	"golang.org/x/sys/unix"
)

const (
	device    = "/sys/class/power_supply/BAT?"
	threshold = "charge_control_end_threshold"
	services  = "/etc/systemd/system/"
)

var events = [...]string{
	"hibernate",
	"hybrid-sleep",
	"multi-user",
	"suspend",
	"suspend-then-hibernate",
}

var build, tag string

var (
	//go:embed bat.service
	unit string
	//go:embed help.fmt
	help string
	//go:embed version.fmt
	version string
)

func usage() {
	t, err := time.Parse("2006-01-02", build)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stdout, help, t.Format("02 January 2006"))
}

func main() {
	if len(os.Args) == 1 {
		usage()
		return
	}

	batteries, err := filepath.Glob(filepath.Join(device, threshold))
	if err != nil {
		log.Fatal(err)
	}
	if len(batteries) == 0 {
		fmt.Fprintln(os.Stderr, "This program is most likely not compatible with your system. See\nhttps://github.com/tshakalekholoane/bat#disclaimer for details.")
		os.Exit(1)
	}
	first, _ := filepath.Split(batteries[0])
	if len(batteries) > 1 {
		fmt.Fprintln(os.Stderr, "More than 1 battery device found, using " + first)
	}
	data := make([]byte, 32)
	mustRead := func(variable string) string {
		f, err := os.Open(filepath.Join(first, variable))
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		n, err := f.Read(data)
		if err != nil && err != io.EOF {
			log.Fatal(err)
		}
		return string(data[:n-1])
	}
	switch option := os.Args[1]; option {
	case "-h", "--help":
		usage()
	case "-v", "--version":
		fmt.Fprintf(os.Stdout, version, tag, time.Now().Year())
	case "capacity", "status":
		fmt.Fprintln(os.Stdout, mustRead(option))
	case "health":
		// Some devices use charge_* and others energy_* so probe both. The
		// health is computed as v / w where v is the eroded capacity and w
		//	is the capacity when the battery was new.
		enoent := false
		a, b := "charge_full", "charge_full_design"
		_, err := os.Stat(filepath.Join(first, a))
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				log.Fatal(err)
			}
			enoent = true
		}
		_, err = os.Stat(filepath.Join(first, b))
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				log.Fatal(err)
			}
			enoent = true
		}
		if enoent {
			a, b = "energy_full", "energy_full_design"
		}
		v, err := strconv.Atoi(mustRead(a))
		if err != nil {
			log.Fatal(err)
		}
		w, err := strconv.Atoi(mustRead(b))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Fprintln(os.Stdout, v*100/w)
	case "persist":
		output, err := exec.Command("systemctl", "--version").CombinedOutput()
		if err != nil {
			log.Fatal(err)
		}
		var version int
		_, err = fmt.Sscanf(string(output), "systemd %d", &version)
		if err != nil {
			log.Fatal(err)
		}
		// systemd 244-rc1 is the earliest version to allow restarts for
		// oneshot services.
		if version < 244 {
			fmt.Fprintln(os.Stderr, "Requires systemd version 243-rc1 or later.")
			os.Exit(1)
		}

		current, err := strconv.Atoi(mustRead(threshold))
		if err != nil {
			log.Fatal(err)
		}

		shell, err := exec.LookPath("sh")
		if err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				fmt.Fprintln(os.Stderr, "Could not find 'sh' on your system.")
				os.Exit(1)
			}
			log.Fatal(err)
		}

		for _, event := range events {
			tmpl, err := template.New("unit").Parse(unit)
			if err != nil {
				log.Fatal(err)
			}

			name := services + "bat-" + event + ".service"
			f, err := os.Create(name)
			if err != nil {
				if errors.Is(err, syscall.EACCES) {
					fmt.Fprintln(os.Stderr, "Permission denied. Try running this command using sudo.")
					os.Exit(1)
				}
				log.Fatal(err)
			}
			defer f.Close()

			service := struct {
				Event     string
				Shell     string
				Path      string
				Threshold int
			}{event, shell, batteries[0], current}
			if err := tmpl.Execute(f, service); err != nil {
				log.Fatal(err)
			}

			cmd := exec.Command("systemctl", "enable", name)
			if err := cmd.Run(); err != nil {
				log.Fatal(err)
			}
		}
		fmt.Fprintln(os.Stdout, "Persistence of the current charging threshold enabled.")
	case "threshold":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stdout, mustRead(threshold))
		} else {
			t := os.Args[2]
			v, err := strconv.Atoi(t)
			if err != nil {
				if errors.Is(err, strconv.ErrSyntax) {
					fmt.Fprintln(os.Stderr, "Argument should be an integer.")
					os.Exit(1)
				}
				log.Fatal(err)
			}

			if v < 1 || v > 100 {
				fmt.Fprintln(os.Stderr, "Threshold value should be between 1 and 100.")
				os.Exit(1)
			}

			var utsname unix.Utsname
			if err = unix.Uname(&utsname); err != nil {
				log.Fatal(err)
			}
			var maj, min int
			_, err = fmt.Sscanf(string(utsname.Release[:]), "%d.%d", &maj, &min)
			if err != nil {
				log.Fatal(err)
			}

			// The earliest version of the Linux kernel to expose the battery
			// charging threshold is 5.4.
			if maj <= 5 && (maj != 5 || min < 4) {
				fmt.Fprintf(os.Stderr, "Requires Linux kernel version 5.4 or later.")
				os.Exit(1)
			}

			if err := os.WriteFile(batteries[0], []byte(t), 0o644); err != nil {
				if errors.Is(err, syscall.EACCES) {
					fmt.Fprintln(os.Stderr, "Permission denied. Try running this command using sudo.")
					os.Exit(1)
				}
				log.Fatal(err)
			}
			fmt.Fprintln(os.Stdout, "Charging threshold set.\nRun `sudo bat persist` to persist the setting between restarts.")
		}
	case "reset":
		for _, event := range events {
			name := services + "bat-" + event + ".service"
			buf := new(bytes.Buffer)
			cmd := exec.Command("systemctl", "disable", name)
			cmd.Stderr = buf
			if err := cmd.Run(); err != nil {
				switch message := buf.String(); {
				case strings.Contains(message, "does not exist"):
					continue
				case strings.Contains(message, "Access denied"):
					fmt.Fprintln(os.Stderr, "Permission denied. Try running this command using sudo.")
					os.Exit(1)
				default:
					log.Fatal(err)
				}
			}
			if err := os.Remove(name); err != nil && !errors.Is(err, syscall.ENOENT) {
				log.Fatal(err)
			}
		}
		fmt.Fprintln(os.Stdout, "Charging threshold persistence reset.")
	default:
		fmt.Fprintf(os.Stderr, "There is no %s option. Run `bat --help` to see a list of available options.\n", option)
		os.Exit(1)
	}
}
