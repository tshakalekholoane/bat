package variable

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestVal(t *testing.T) {
	tests := [...]struct {
		variable Variable
		want     string
	}{
		{Capacity, "79"},
		{Status, "Not charging"},
		{Threshold, "80"},
	}

	for _, test := range tests {
		t.Run(test.variable.String(), func(t *testing.T) {
			dir = os.TempDir()

			f, err := os.Create(filepath.Join(dir, test.variable.String()))
			assert.NilError(t, err)
			defer os.Remove(f.Name())

			_, err = f.Write([]byte(test.want))
			assert.NilError(t, err)

			got, err := Val(test.variable)
			assert.NilError(t, err)
			assert.Equal(t, got, test.want)

			assert.NilError(t, f.Close())
		})
	}
}
