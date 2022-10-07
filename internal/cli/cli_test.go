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

func (s *status) reset() {
	s.code = 0
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

type serviceErr uint8

const (
	ok = iota
	bashNotFoundErr
	incompatSystemdErr
	varNotFoundErr
	eacces
)

type testService struct {
	sim serviceErr
}

// newTestService returns an object that conforms to services.Servicer
// which will simulate the specified error.
func newTestService(sim serviceErr) *testService {
	return &testService{sim: sim}
}

func (ts *testService) Write() error {
	switch ts.sim {
	case bashNotFoundErr:
		return services.ErrBashNotFound
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

func (ts *testService) Delete() error {
	if ts.sim == eacces {
		return syscall.EACCES
	}
	return nil
}

func newTestApp(cons *console) *app {
	return &app{
		console: cons,
		pager:   "less",
		read:    testVal,
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

	t.Run("app.help() output == help.txt", func(t *testing.T) {
		app.help()

		got := cons.out.(*bytes.Buffer).String()
		want := help

		assert.Equal(t, got, want)
		assert.Equal(t, stat.code, success, "exit status = %d, want %d", stat.code, success)
	})

	t.Run("app.help() output != help.txt", func(t *testing.T) {
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

	t.Run("app.version() output == version.tmpl", func(t *testing.T) {
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
		{"func (a *app) capacity()", app.capacity, "79\n", success},
		{"func (a *app) status()", app.status, "Not charging\n", success},
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
		sim  serviceErr
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
			app.service = newTestService(test.sim)

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
		sim  serviceErr
		msg  string
		code int
	}{
		{ok, persistenceReset, success},
		{eacces, permissionDenied, failure},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("app.reset() = %q", test.msg), func(t *testing.T) {
			app.service = newTestService(test.sim)

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
