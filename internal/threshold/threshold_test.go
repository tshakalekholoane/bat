package threshold

import (
	"fmt"
	"testing"

	"gotest.tools/v3/assert"
)

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
		t.Run(fmt.Sprintf("isRequiredKernel(%q)", test.input), func(t *testing.T) {
			got, err := isRequiredKernel(test.input)
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
		t.Run(fmt.Sprintf("IsInvalid(%d)", test.input), func(t *testing.T) {
			got := IsValid(test.input)
			assert.Equal(t, got, test.want)
		})
	}
}
