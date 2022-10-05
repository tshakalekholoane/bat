package cli

import (
	"bytes"
	"fmt"
	"os/exec"
	"testing"
	"text/template"
	"time"

	"gotest.tools/v3/assert"
	"tshaka.co/bat/internal/variable"
)

// status spies on the exit function to ensure the correct exit code is
// returned.
type status struct {
	code int
}

func (s *status) set(code int) {
	s.code = code
}

func newTestConsole() (*status, *console) {
	s := &status{}
	c := &console{
		err:   new(bytes.Buffer),
		out:   new(bytes.Buffer),
		pager: "less",
		quit:  s.set,
	}
	return s, c
}

// testVal mocks the variable.Val function.
func testVal(v variable.Variable) (string, error) {
	switch v {
	case variable.Capacity:
		return "79", nil
	case variable.Status:
		return "Not charging", nil
	case variable.Threshold:
		return "80", nil
	default:
		return "", variable.ErrNotFound
	}
}

func TestHelp(t *testing.T) {
	stat, cons := newTestConsole()

	t.Run("cli/console.page(help) output == help.txt", func(t *testing.T) {
		cons.page(help)

		got := cons.out.(*bytes.Buffer).String()
		want := help

		assert.Equal(t, got, want)
		assert.Equal(t, stat.code, success, "exit status = %d, want %d", stat.code, success)
	})

	t.Run("cli/console.page(help) output != help.txt", func(t *testing.T) {
		cons.page(help)

		got := cons.out.(*bytes.Buffer).String()
		want := help[1:]

		assert.Assert(t, got != want, "cli.page(help) output == help.txt")
		assert.Equal(t, stat.code, success, "exit status = %d, want %d", stat.code, success)
	})

	t.Run(`cli/console.page("") = fatal error`, func(t *testing.T) {
		// One of the errors that can occur with paging is if the less pager
		// is not in the path.
		cons.pager = ""

		cons.page("")
		got := cons.err.(*bytes.Buffer).Bytes()
		want := []byte("cli: fatal error: ")

		assert.Assert(t, bytes.HasPrefix(got, want), "cli.page output != prefix cli: fatal error")
		assert.Equal(t, stat.code, failure, "exit status = %d, want %d", stat.code, failure)
	})
}

func TestVersion(t *testing.T) {
	stat, cons := newTestConsole()

	t.Run("cli/console.page(ver)", func(t *testing.T) {
		cons.page(info(tag, time.Now()))
		got := cons.out.(*bytes.Buffer)

		cmd := exec.Command("git", "describe", "--always", "--dirty", "--tags", "--long")
		out, err := cmd.Output()
		assert.NilError(t, err)

		want := new(bytes.Buffer)
		tmpl := template.Must(template.New("version").Parse(version))
		tmpl.Execute(want, struct {
			Tag  string
			Year int
		}{
			string(bytes.TrimSpace(out)),
			time.Now().Year(),
		})

		assert.Assert(t, bytes.Contains(want.Bytes(), got.Bytes()))
		assert.Equal(t, stat.code, success, "exit status = %d, want %d", stat.code, success)
	})
}

func TestShow(t *testing.T) {
	const newline = "\n"
	tests := [...]struct {
		val  variable.Variable
		want string
		code int
	}{
		{variable.Capacity, "79" + newline, success},
		{variable.Status, "Not charging" + newline, success},
		{variable.Threshold, "80" + newline, success},
		{0, incompat + newline, failure},
	}

	stat, cons := newTestConsole()
	app := &app{
		console: cons,
		read:    testVal,
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("show(%q) = %q", test.val.String(), test.want), func(t *testing.T) {
			app.show(test.val)

			assert.Equal(t, stat.code, test.code, "exit status = %d, want %d", stat.code, test.code)

			var buf *bytes.Buffer
			if stat.code == success {
				buf = app.console.out.(*bytes.Buffer)
			} else {
				buf = app.console.err.(*bytes.Buffer)
			}

			got := buf.String()
			assert.Equal(t, got, test.want)

			buf.Reset()
		})
	}
}
