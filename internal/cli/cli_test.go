package cli

import (
	"bytes"
	"fmt"
	"io/fs"
	"os/exec"
	"syscall"
	"testing"
	"text/template"
	"time"

	"gotest.tools/v3/assert"
	"tshaka.co/x/bat/internal/systemd"
	"tshaka.co/x/bat/pkg/power"
)

// status spies on the exit function to ensure the correct exit code is
// returned.
type status struct{ code int }

func (s *status) set(code int) { s.code = code }

// get mocks the power.Get function.
func get(v power.Variable) (string, error) {
	switch v {
	case power.Capacity:
		return "79", nil
	case power.Status:
		return "Not charging", nil
	case power.Threshold:
		return "80", nil
	default:
		return "", power.ErrNotFound
	}
}

// setter implements a method that mocks the power.Set function. It has
// an error field which can be used to simulate an error from the actual
// function for testing.
type setter struct{ err error }

func (s *setter) set(v power.Variable, val string) error { return s.err }

// testSystemd mocks systemd.Systemd by implementing resetwriter. It
// takes an err field that can be used to simulate errors from the
// actual methods for testing.
type testSystemd struct{ err error }

func (ts testSystemd) Reset() error { return ts.err }
func (ts testSystemd) Write() error { return ts.err }

func TestIsRequiredKernel(t *testing.T) {
	tests := [...]struct {
		input string
		want  bool
	}{
		{"4.0.9", false},
		{"4.1.52", false},
		{"4.4.302", false},
		{"4.19.245", false},
		{"5.0.21", false},
		{"5.3.18", false},
		{"5.4.196", true},
		{"5.10.118", true},
		{"5.2-rc2", false},
		{"5.4-rc5", true},
		{"5.15.0-2-amd64", true},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("requiredKernel(%q)", test.input), func(t *testing.T) {
			got, err := requiredKernel(test.input)
			assert.NilError(t, err, "parse version string: %s", test.input)

			assert.Equal(t, got, test.want)
		})
	}
}

func TestInvalid(t *testing.T) {
	tests := [...]struct {
		input int
		want  bool
	}{
		{-1, false},
		{0, false},
		{1, true},
		{2, true},
		{100, true},
		{101, false},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("valid(%d)", test.input), func(t *testing.T) {
			got := valid(test.input)
			assert.Equal(t, got, test.want)
		})
	}
}

func TestHelp(t *testing.T) {
	status := &status{}
	app := &app{
		console: &console{
			out:  new(bytes.Buffer),
			quit: status.set,
		},
		pager: "less",
	}

	t.Run("app.help() == help.txt", func(t *testing.T) {
		app.help()

		got := app.console.out.(*bytes.Buffer).String()
		want := help

		assert.Equal(t, got, want)
		assert.Equal(t, status.code, success, "exit status = %d, want %d", status.code, success)
	})

	t.Run("app.help() != help.txt", func(t *testing.T) {
		app.help()

		got := app.console.out.(*bytes.Buffer).String()
		want := help[1:]

		assert.Assert(t, got != want, "cli.page(help) output == help.txt")
		assert.Equal(t, status.code, success, "exit status = %d, want %d", status.code, success)
	})
}

func TestVersion(t *testing.T) {
	status := &status{}
	app := &app{
		console: &console{
			out:  new(bytes.Buffer),
			quit: status.set,
		},
		pager: "less",
	}

	t.Run("app.version() == version.tmpl", func(t *testing.T) {
		app.version()
		got := app.console.out.(*bytes.Buffer)

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
		assert.Equal(t, status.code, success, "exit status = %d, want %d", status.code, success)
	})
}

func TestShow(t *testing.T) {
	status := &status{}
	app := &app{
		console: &console{
			out:  new(bytes.Buffer),
			quit: status.set,
		},
		get: get,
	}

	tests := [...]struct {
		name string
		fn   func()
		want string
		code int
	}{
		{"app.capacity()", app.capacity, "79\n", success},
		{"app.status()", app.status, "Not charging\n", success},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s = %q", test.name, test.want), func(t *testing.T) {
			test.fn()

			assert.Equal(t, status.code, test.code, "exit status = %d, want %d", status.code, test.code)

			buf := app.console.out.(*bytes.Buffer)

			got := buf.String()
			assert.Equal(t, got, test.want)

			buf.Reset()
		})
	}
}

func TestPersist(t *testing.T) {
	status := &status{}
	app := &app{
		console: &console{
			err:  new(bytes.Buffer),
			out:  new(bytes.Buffer),
			quit: status.set,
		},
	}

	tests := [...]struct {
		err  error
		msg  string
		code int
	}{
		{nil, msgPersistenceEnabled, success},
		{systemd.ErrBashNotFound, msgBashNotFound, failure},
		{systemd.ErrIncompatSystemd, msgIncompatibleSystemd, failure},
		{power.ErrNotFound, msgIncompatible, failure},
		{&fs.PathError{Err: syscall.EACCES}, msgPermissionDenied, failure},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("app.persist() = %q", test.msg), func(t *testing.T) {
			app.systemder = &testSystemd{test.err}

			app.persist()

			assert.Equal(t, status.code, test.code, "exit status = %d, want %d", status.code, test.code)

			var buf *bytes.Buffer
			if status.code == success {
				buf = app.console.out.(*bytes.Buffer)
			} else {
				buf = app.console.err.(*bytes.Buffer)
			}

			got := buf.String()
			want := test.msg + "\n"

			assert.Equal(t, got, want)

			buf.Reset()
		})
	}
}

func TestReset(t *testing.T) {
	status := &status{}
	app := &app{
		console: &console{
			err:  new(bytes.Buffer),
			out:  new(bytes.Buffer),
			quit: status.set,
		},
	}

	tests := [...]struct {
		err  error
		msg  string
		code int
	}{
		{nil, msgPersistenceReset, success},
		{&fs.PathError{Err: syscall.EACCES}, msgPermissionDenied, failure},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("app.reset() = %q", test.msg), func(t *testing.T) {
			app.systemder = &testSystemd{test.err}

			app.reset()

			assert.Equal(t, status.code, test.code, "exit status = %d, want %d", status.code, test.code)

			var buf *bytes.Buffer
			if status.code == success {
				buf = app.console.out.(*bytes.Buffer)
			} else {
				buf = app.console.err.(*bytes.Buffer)
			}

			got := buf.String()
			want := test.msg + "\n"

			assert.Equal(t, got, want)

			buf.Reset()
		})
	}
}

func TestThreshold(t *testing.T) {
	status := &status{}
	app := &app{
		console: &console{
			err:  new(bytes.Buffer),
			out:  new(bytes.Buffer),
			quit: status.set,
		},
	}

	tests := [...]struct {
		args []string
		code int
		err  error
		want string
	}{
		{[]string{"bat", "threshold", "80"}, success, nil, msgThresholdSet},
		{[]string{"bat", "threshold", "80", "extraneous_arg"}, failure, nil, msgExpectedSingleArg},
		{[]string{"bat", "threshold", "80.0"}, failure, nil, msgArgNotInt},
		{[]string{"bat", "threshold", "101"}, failure, nil, msgOutOfRangeThresholdVal},
		{[]string{"bat", "threshold", "80"}, failure, power.ErrNotFound, msgIncompatible},
		{[]string{"bat", "threshold", "80"}, failure, &fs.PathError{Err: syscall.EACCES}, msgPermissionDenied},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("app.threshold() = %q", test.want), func(t *testing.T) {
			ts := setter{test.err}
			app.set = ts.set

			app.threshold(test.args)

			assert.Equal(t, status.code, test.code, "exit status = %d, want %d", status.code, test.code)

			var buf *bytes.Buffer
			if status.code == success {
				buf = app.console.out.(*bytes.Buffer)
			} else {
				buf = app.console.err.(*bytes.Buffer)
			}

			got := buf.String()
			want := test.want + "\n"

			assert.Equal(t, got, want)

			buf.Reset()
		})
	}
}
