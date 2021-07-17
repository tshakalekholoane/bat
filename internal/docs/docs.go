package docs

import (
    _ "embed"
    "os"
    "os/exec"
    "strings"
)

//go:embed help.txt
var help string

//go:embed version.txt
var version string

// page calls the less pager on the `doc` string input. A successful 
// call returns err == nil.
func page(doc string) error {
    cmd := exec.Command("less", "--IGNORE-CASE", "--quit-if-one-screen")
    cmd.Stdin = strings.NewReader(doc)
    cmd.Stdout = os.Stdout
    err := cmd.Run()
    if err != nil {
        return err
    }
    return nil
}

// Help shows the help document through the less pager. A successful
// call returns err == nil.
func Help() error {
    err := page(help)
    if err != nil {
        return err
    }
    return nil
}

// VersionInfo shows version information through the less pager. A
// successful call returns err == nil.
func VersionInfo() error {
    err := page(version)
    if err != nil {
        return err
    }
    return nil
}
