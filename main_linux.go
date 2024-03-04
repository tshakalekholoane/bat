// Binary bat is a battery management utility for Linux laptops.
package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"text/template"

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
	//go:embed help.txt
	help string
	//go:embed version.fmt
	version string
)

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func read(battery, variable string) (string, error) {
	f, err := os.Open(filepath.Join(battery, variable))
	if err != nil {
		return "", err
	}
	defer f.Close()
	buf := make([]byte, 32)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}
	return string(buf[:n-1]), nil
}

type options struct{ debug, help, version bool }

func main() {
	var opts options
	for _, arg := range os.Args {
		switch arg {
		case "-d", "--debug":
			opts.debug = true
		case "-h", "--help":
			opts.help = true
		case "-v", "--version":
			opts.version = true
		}
	}

	if len(os.Args) == 1 {
		fmt.Fprint(os.Stderr, help)
		os.Exit(2)
	}

	if opts.help {
		fmt.Print(help)
		return
	}

	if opts.version {
		fmt.Printf(version, tag)
		return
	}

	defer func() {
		if err := recover(); err != nil {
			if opts.debug {
				fmt.Fprintf(os.Stderr, "%s\n\n%s", err, string(debug.Stack()))
			} else {
				fmt.Println("A fatal error occurred. Please rerun the command with the `--debug` flag enabled\nand file an issue with the resulting output at the following address\nhttps://github.com/tshakalekholoane/bat/issues/new.")
			}
		}
	}()

	batteries := must(filepath.Glob(device))
	if len(batteries) == 0 {
		fmt.Fprintln(os.Stderr, "This program is most likely not compatible with your system. See\nhttps://github.com/tshakalekholoane/bat#disclaimer for details.")
		os.Exit(1)
	}

	// Default to using the first.
	bat := batteries[0]

	switch variable := os.Args[1]; variable {
	case "capacity", "status":
		fmt.Println(must(read(bat, variable)))
	case "health":
		// Some devices use charge_* and others energy_* so probe both. The
		// health is computed as v / w where v is the eroded capacity and w
		// is the capacity when the battery was new.
		var enoent bool
		x, y := "charge_full", "charge_full_design"
		_, err := os.Stat(filepath.Join(bat, x))
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				panic(err)
			}
			enoent = true
		}
		_, err = os.Stat(filepath.Join(bat, y))
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				panic(err)
			}
			enoent = true
		}
		if enoent {
			x, y = "energy_full", "energy_full_design"
		}
		v := must(strconv.Atoi(must(read(bat, x))))
		w := must(strconv.Atoi(must(read(bat, y))))
		fmt.Println(v * 100 / w)
	case "persist":
		// systemd 244-rc1 is the earliest version to allow restarts for
		// oneshot services.
		out := must(exec.Command("systemctl", "--version").CombinedOutput())
		var rev int
		_ = must(fmt.Sscanf(string(out), "systemd %d", &rev))
		if rev < 244 {
			fmt.Fprintln(os.Stderr, "Requires systemd version 243-rc1 or later.")
			os.Exit(1)
		}

		curr := must(strconv.Atoi(must(read(bat, threshold))))
		sh, err := exec.LookPath("sh")
		if err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				fmt.Fprintln(os.Stderr, "Could not find 'sh' on your system.")
				os.Exit(1)
			}
			panic(err)
		}
		p := path.Join(bat, threshold)
		tmpl := template.Must(template.New("unit").Parse(unit))
		for _, event := range events {
			name := services + "bat-" + event + ".service"
			f, err := os.Create(name)
			if err != nil {
				if errors.Is(err, syscall.EACCES) {
					fmt.Fprintln(os.Stderr, "Permission denied. Try running this command using sudo.")
					os.Exit(1)
				}
				panic(err)
			}
			defer f.Close()
			svc := struct {
				Event, Path, Shell string
				Threshold          int
			}{event, p, sh, curr}
			if err := tmpl.Execute(f, svc); err != nil {
				panic(err)
			}
			if err := exec.Command("systemctl", "enable", name).Run(); err != nil {
				panic(err)
			}
		}
		fmt.Println("Persistence of the current charging threshold enabled.")
	case "threshold":
		if len(os.Args) < 3 {
			// Get.
			fmt.Println(must(read(bat, threshold)))
		} else {
			// Set.
			// The earliest version of the Linux kernel to expose the battery
			// charging threshold is 5.4.
			var utsname unix.Utsname
			if err := unix.Uname(&utsname); err != nil {
				panic(err)
			}
			var maj, min int
			_ = must(fmt.Sscanf(string(utsname.Release[:]), "%d.%d", &maj, &min))
			if maj <= 5 && (maj != 5 || min < 4) {
				fmt.Fprintf(os.Stderr, "Requires Linux kernel version 5.4 or later.")
				os.Exit(1)
			}

			a := os.Args[2]
			i, err := strconv.Atoi(a)
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

			if err := os.WriteFile(filepath.Join(bat, threshold), []byte(a), 0o644); err != nil {
				if errors.Is(err, syscall.EACCES) {
					fmt.Fprintln(os.Stderr, "Permission denied. Try running this command using sudo.")
					os.Exit(1)
				}
				panic(err)
			}
			fmt.Println("Charging threshold set.\nRun `sudo bat persist` to persist the setting between restarts.")
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
					panic(err)
				}
			}
			if err := os.Remove(name); err != nil && !errors.Is(err, syscall.ENOENT) {
				if errors.Is(err, syscall.EACCES) {
					fmt.Fprintln(os.Stderr, "Permission denied. Try running this command using sudo.")
					os.Exit(1)
				}
				panic(err)
			}
		}
		fmt.Println("Charging threshold persistence reset.")
	default:
		fmt.Fprintf(os.Stderr, "There is no %s command. Run `bat --help` to see a list of available commands.\n", variable)
		os.Exit(1)
	}
}
