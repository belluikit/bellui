package commands

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// With makeappx.exe on PATH, findWindowsSDKBin returns its directory: both
// callers join "makeappx.exe" or "signtool.exe" onto the result, so returning
// the executable's own path made them look for makeappx.exe/makeappx.exe (#337).
func TestFindWindowsSDKBinFromPath(t *testing.T) {
	sdk := filepath.Join("sdk", "bin", "x64")
	old := lookPath
	lookPath = func(file string) (string, error) {
		return filepath.Join(sdk, file), nil
	}
	t.Cleanup(func() { lookPath = old })

	dir, err := findWindowsSDKBin()
	require.NoError(t, err)
	assert.Equal(t, sdk, dir)
}
