package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func execute(t *testing.T, s string) string {
	t.Helper()
	parts := strings.Split(s, " ")
	out, err := exec.Command(parts[0], parts[1:]...).CombinedOutput()
	assert.NilError(t, err)
	return string(bytes.TrimSpace(out))
}

func TestProgInfo(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"tag", "git describe --always --dirty --tags --long", tag},
		{"build", "date -u +%Y-%m-%d", build},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := execute(t, test.input)
			assert.Equal(t, got, test.want)
		})
	}
}
