// Package docs implements functions for displaying documentation.
package docs

import (
	_ "embed"
	"os"
	"os/exec"
	"strings"
)

var (
	//go:embed help.txt
	help string
	//go:embed version.txt
	version string
)

// page filters the string doc through the less pager.
func page(doc string) error {
	cmd := exec.Command(
		"less", "--no-init", "--quit-if-one-screen", "--IGNORE-CASE")
	cmd.Stdin = strings.NewReader(doc)
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

// Usage pages the help documentation through less.
func Usage() error {
	err := page(help)
	if err != nil {
		return err
	}
	return nil
}

// Version pages version information through less.
func Version() error {
	err := page(version)
	if err != nil {
		return err
	}
	return nil
}
