package cli

import (
	"bytes"
	"testing"

	"gotest.tools/v3/assert"
)

// status spies on the exit function to ensure the correct exit code is
// returned.
type status struct {
	code int
}

func (s *status) set(code int) {
	s.code = code
}

func TestHelp(t *testing.T) {
	s := status{}
	c := console{
		err:   new(bytes.Buffer),
		out:   new(bytes.Buffer),
		pager: "less",
		quit:  s.set,
	}

	t.Run("cli.page(help) output == help.txt", func(t *testing.T) {
		c.page(help)

		got := c.out.(*bytes.Buffer).String()
		want := help

		assert.Equal(t, got, want)
		assert.Equal(t, s.code, success, "exit status = %d, want %d", s.code, success)
	})

	t.Run("cli.page(help) output != help.txt", func(t *testing.T) {
		c.page(help)

		got := c.out.(*bytes.Buffer).String()
		want := help[1:]

		assert.Assert(t, got != want, "cli.page(help) output == help.txt")
		assert.Equal(t, s.code, success, "exit status = %d, want %d", s.code, success)
	})

	t.Run(`cli.page("") = fatal error`, func(t *testing.T) {
		// One of the errors that can occur with paging is if the less pager
		// is not in the path.
		c.pager = ""

		c.page("")
		got := c.err.(*bytes.Buffer).Bytes()
		want := []byte("cli: fatal error: ")

		assert.Assert(t, bytes.HasPrefix(got, want), "cli.page output != prefix cli: fatal error")
		assert.Equal(t, s.code, failure, "exit status = %d, want %d", s.code, failure)
	})
}
