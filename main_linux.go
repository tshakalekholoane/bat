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

type battery struct {
	path string
	buf  []byte
}

func (b *battery) read(v string) string {
	f, err := os.Open(filepath.Join(b.path, v))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	n, err := f.Read(b.buf)
	if err != nil && err != io.EOF {
		panic(err)
	}
	return string(b.buf[:n-1])
}

func (b *battery) capacity() string  { return b.read("capacity") }
func (b *battery) status() string    { return b.read("status") }
func (b *battery) threshold() string { return b.read(threshold) }

func (b *battery) health() int {
	// Some devices use charge_* and others energy_* so probe both. The
	// health is computed as v / w where v is the eroded capacity and w is
	// the capacity when the battery was new.
	enoent := false
	x, y := "charge_full", "charge_full_design"
	_, err := os.Stat(filepath.Join(b.path, x))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			panic(err)
		}
		enoent = true
	}
	_, err = os.Stat(filepath.Join(b.path, y))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			panic(err)
		}
		enoent = true
	}
	if enoent {
		x, y = "energy_full", "energy_full_design"
	}
	v, err := strconv.Atoi(b.read(x))
	if err != nil {
		panic(err)
	}
	w, err := strconv.Atoi(b.read(y))
	if err != nil {
		panic(err)
	}
	return v * 100 / w
}

func (b *battery) setThreshold(v string) error {
	err := os.WriteFile(filepath.Join(b.path, threshold), []byte(v), 0o644)
	if err != nil {
		return err
	}
	return nil
}

var usage = func() string {
	// Requires a build script to set the build time.
	date, err := time.Parse("2006-01-02", build)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf(help, date.Format("02 January 2006"))
}()

type options struct{ debug, help, version bool }

func main() {
	opts := &options{}
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
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	if opts.help {
		fmt.Print(usage)
		return
	}

	if opts.version {
		fmt.Printf(version, tag, time.Now().Year())
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

	batteries, err := filepath.Glob(device)
	if err != nil {
		panic(err)
	}
	if len(batteries) == 0 {
		fmt.Fprintln(os.Stderr, "This program is most likely not compatible with your system. See\nhttps://github.com/tshakalekholoane/bat#disclaimer for details.")
		os.Exit(1)
	}

	batt := &battery{
		path: batteries[0],
		buf:  make([]byte, 32),
	}

	switch command := os.Args[1]; command {
	case "capacity":
		fmt.Println(batt.capacity())
	case "status":
		fmt.Println(batt.status())
	case "health":
		fmt.Println(batt.health())
	case "persist":
		out, err := exec.Command("systemctl", "--version").CombinedOutput()
		if err != nil {
			panic(err)
		}
		var rev int
		_, err = fmt.Sscanf(string(out), "systemd %d", &rev)
		if err != nil {
			panic(err)
		}
		// systemd 244-rc1 is the earliest version to allow restarts for
		// oneshot services.
		if rev < 244 {
			fmt.Fprintln(os.Stderr, "Requires systemd version 243-rc1 or later.")
			os.Exit(1)
		}
		current, err := strconv.Atoi(batt.threshold())
		if err != nil {
			panic(err)
		}
		sh, err := exec.LookPath("sh")
		if err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				fmt.Fprintln(os.Stderr, "Could not find 'sh' on your system.")
				os.Exit(1)
			}
			panic(err)
		}
		path := path.Join(batt.path, threshold)
		for _, event := range events {
			tmpl := template.Must(template.New("unit").Parse(unit))
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
			service := struct {
				Event, Path, Shell string
				Threshold          int
			}{event, path, sh, current}
			if err := tmpl.Execute(f, service); err != nil {
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
			fmt.Println(batt.threshold())
		} else {
			// Set.
			var utsname unix.Utsname
			if err = unix.Uname(&utsname); err != nil {
				panic(err)
			}
			var maj, min int
			_, err = fmt.Sscanf(string(utsname.Release[:]), "%d.%d", &maj, &min)
			if err != nil {
				panic(err)
			}
			// The earliest version of the Linux kernel to expose the battery
			// charging threshold is 5.4.
			if maj <= 5 && (maj != 5 || min < 4) {
				fmt.Fprintf(os.Stderr, "Requires Linux kernel version 5.4 or later.")
				os.Exit(1)
			}

			v := os.Args[2]
			w, err := strconv.Atoi(v)
			if err != nil {
				if errors.Is(err, strconv.ErrSyntax) {
					fmt.Fprintln(os.Stderr, "Argument should be an integer.")
					os.Exit(1)
				}
				panic(err)
			}
			if w < 1 || w > 100 {
				fmt.Fprintln(os.Stderr, "Threshold value should be between 1 and 100.")
				os.Exit(1)
			}

			if err := batt.setThreshold(v); err != nil {
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
				panic(err)
			}
		}
		fmt.Println("Charging threshold persistence reset.")
	default:
		fmt.Fprintf(os.Stderr, "There is no %s option. Run `bat --help` to see a list of available options.\n", command)
		os.Exit(1)
	}
}
