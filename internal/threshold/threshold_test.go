package threshold

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func FuzzSet(f *testing.F) {
	tests := [...]int{-1, 0, 1, 99, 100, 101}
	for _, test := range tests {
		f.Add(test)
	}

	f.Fuzz(func(t *testing.T, want int) {
		if !IsValid(want) {
			return
		}

		file, err := os.CreateTemp("", "charge_threshold")
		assert.NilError(t, err)
		defer os.Remove(file.Name())

		// Reassign charging threshold variable path for testing.
		threshold = file.Name()

		err = Set(want)
		assert.NilError(t, err, "set charging threshold: %v", err)

		b := make([]byte, 3)
		_, err = file.Read(b)
		assert.NilError(t, err, "read threshold value: %v", err)

		got, err := strconv.Atoi(strings.TrimRight(string(b), "\x00"))
		assert.NilError(t, err, "convert byte string to int: %v", err)

		assert.Equal(t, got, want)
	})
}

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

func TestSet(t *testing.T) {
	f, err := os.CreateTemp("", "charge_threshold")
	assert.NilError(t, err)
	defer os.Remove(f.Name())

	// Reassign charging threshold variable path for testing.
	threshold = f.Name()

	// Generate random threshold value.
	rand.Seed(time.Now().UnixNano())
	want := rand.Intn(101) + 1

	t.Run(fmt.Sprintf("Set(%v)", want), func(t *testing.T) {
		err := Set(want)
		assert.NilError(t, err, "set charging threshold: %v", err)

		b := make([]byte, 3)
		_, err = f.Read(b)
		assert.NilError(t, err, "read threshold value: %v", err)

		got, err := strconv.Atoi(strings.TrimRight(string(b), "\x00"))
		assert.NilError(t, err, "convert byte string to int: %v", err)

		assert.Equal(t, got, want)
	})
}
