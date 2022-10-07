package cli

import (
	"bytes"
	"fmt"
	"os/exec"
	"syscall"
	"testing"
	"text/template"
	"time"

	"gotest.tools/v3/assert"
	"tshaka.co/bat/internal/services"
	"tshaka.co/bat/internal/threshold"
	"tshaka.co/bat/internal/variable"
)

// testGet mocks the variable.Get function.
func testGet(v variable.Variable) (string, error) {
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

// status spies on the exit function to ensure the correct exit code is
// returned.
type status struct {
	code int
}

func (s *status) set(code int) {
	s.code = code
}

func (s *status) reset() {
	s.code = 0
}

type simulatedError uint8

const (
	ok simulatedError = iota
	bashNotFoundErr
	eacces
	incompatKernelErr
	incompatSystemdErr
	varNotFoundErr
)

// simulatedError is used to mock functions that return an error.
func simulateError(err simulatedError) error {
	switch err {
	case bashNotFoundErr:
		return services.ErrBashNotFound
	case incompatKernelErr:
		return threshold.ErrIncompatKernel
	case incompatSystemdErr:
		return services.ErrIncompatSystemd
	case varNotFoundErr:
		return variable.ErrNotFound
	case eacces:
		return syscall.EACCES
	default:
		return nil
	}
}

// testSet represents a struct that has a function that with the same
// signature as the one used to set the charging threshold. The sim
// argument is used to instruct which error to simulate.
type testSet struct {
	sim simulatedError
}

func (ts *testSet) set(lvl int) error {
	return simulateError(ts.sim)
}

// testService represents a struct with methods that conforms to
// services.Servicer which will also simulate the specified error.
type testService struct {
	sim simulatedError
}

func (ts *testService) Write() error {
	return simulateError(ts.sim)
}

func (ts *testService) Delete() error {
	return simulateError(ts.sim)
}

func newTestApp(cons *console) *app {
	return &app{
		console: cons,
		pager:   "less",
		get:     testGet,
	}
}

func newTestConsole() (*status, *console) {
	s := &status{}
	c := &console{
		err:  new(bytes.Buffer),
		out:  new(bytes.Buffer),
		quit: s.set,
	}
	return s, c
}

func TestHelp(t *testing.T) {
	stat, cons := newTestConsole()
	app := newTestApp(cons)

	t.Run("app.help() == help.txt", func(t *testing.T) {
		app.help()

		got := cons.out.(*bytes.Buffer).String()
		want := help

		assert.Equal(t, got, want)
		assert.Equal(t, stat.code, success, "exit status = %d, want %d", stat.code, success)
	})

	t.Run("app.help() != help.txt", func(t *testing.T) {
		app.help()

		got := cons.out.(*bytes.Buffer).String()
		want := help[1:]

		assert.Assert(t, got != want, "cli.page(help) output == help.txt")
		assert.Equal(t, stat.code, success, "exit status = %d, want %d", stat.code, success)
	})
}

func TestVersion(t *testing.T) {
	stat, cons := newTestConsole()
	app := newTestApp(cons)

	t.Run("app.version() == version.tmpl", func(t *testing.T) {
		app.version()
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
	stat, cons := newTestConsole()
	app := newTestApp(cons)

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

func TestPersist(t *testing.T) {
	stat, cons := newTestConsole()
	app := newTestApp(cons)

	tests := [...]struct {
		sim  simulatedError
		msg  string
		code int
	}{
		{ok, persistenceEnabled, success},
		{bashNotFoundErr, bashNotFound, failure},
		{incompatSystemdErr, incompatibleSystemd, failure},
		{varNotFoundErr, incompatible, failure},
		{eacces, permissionDenied, failure},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("app.persist() = %q", test.msg), func(t *testing.T) {
			app.service = &testService{sim: test.sim}

			app.persist()

			assert.Equal(t, stat.code, test.code, "exit status = %d, want %d", stat.code, test.code)

			var buf *bytes.Buffer
			if stat.code == success {
				buf = app.console.out.(*bytes.Buffer)
			} else {
				buf = app.console.err.(*bytes.Buffer)
			}

			got := buf.String()
			want := test.msg + "\n"

			assert.Equal(t, got, want)

			stat.reset()
			buf.Reset()
		})
	}
}

func TestReset(t *testing.T) {
	stat, cons := newTestConsole()
	app := newTestApp(cons)

	tests := [...]struct {
		sim  simulatedError
		msg  string
		code int
	}{
		{ok, persistenceReset, success},
		{eacces, permissionDenied, failure},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("app.reset() = %q", test.msg), func(t *testing.T) {
			app.service = &testService{sim: test.sim}

			app.reset()

			assert.Equal(t, stat.code, test.code, "exit status = %d, want %d", stat.code, test.code)

			var buf *bytes.Buffer
			if stat.code == success {
				buf = app.console.out.(*bytes.Buffer)
			} else {
				buf = app.console.err.(*bytes.Buffer)
			}

			got := buf.String()
			want := test.msg + "\n"

			assert.Equal(t, got, want)

			stat.reset()
			buf.Reset()
		})
	}
}

func TestThreshold(t *testing.T) {
	stat, cons := newTestConsole()
	app := newTestApp(cons)

	tests := [...]struct {
		args []string
		code int
		sim  simulatedError
		want string
	}{
		{[]string{"bat", "threshold", "80"}, success, ok, thresholdSet},
		{[]string{"bat", "threshold", "80", "extraneous_arg"}, failure, ok, singleArg},
		{[]string{"bat", "threshold", "80.0"}, failure, ok, argNotInt},
		{[]string{"bat", "threshold", "101"}, failure, ok, outOfRange},
		{[]string{"bat", "threshold", "80"}, failure, incompatKernelErr, incompatibleKernel},
		{[]string{"bat", "threshold", "80"}, failure, varNotFoundErr, incompatible},
		{[]string{"bat", "threshold", "80"}, failure, eacces, permissionDenied},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("app.threshold() = %q", test.want), func(t *testing.T) {
			ts := testSet{sim: test.sim}
			app.set = ts.set

			app.threshold(test.args)

			assert.Equal(t, stat.code, test.code, "exit status = %d, want %d", stat.code, test.code)

			var buf *bytes.Buffer
			if stat.code == success {
				buf = app.console.out.(*bytes.Buffer)
			} else {
				buf = app.console.err.(*bytes.Buffer)
			}

			got := buf.String()
			want := test.want + "\n"

			assert.Equal(t, got, want)

			stat.reset()
			buf.Reset()
		})
	}
}
