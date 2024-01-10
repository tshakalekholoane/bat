package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"tshaka.dev/x/diff"
)

func execute(t *testing.T, s string) string {
	t.Helper()
	parts := strings.Split(s, " ")
	out, err := exec.Command(parts[0], parts[1:]...).CombinedOutput()
	if err != nil {
		t.Error(err)
	}
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
			if got != test.want {
				t.Error(string(diff.Diff("got", []byte(got), "want", []byte(test.want))))
			}
		})
	}
}
