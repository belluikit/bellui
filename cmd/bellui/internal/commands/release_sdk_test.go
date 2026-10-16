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

// Glob returns Windows Kits matches lexically: oldest version first, and arm64
// before x64 and x86. The pick must be the newest SDK whose tools run here.
func TestPickWindowsSDKBin(t *testing.T) {
	bin := func(version, arch string) string {
		return filepath.Join("Windows Kits", "10", "bin", version, arch, "makeappx.exe")
	}
	matches := []string{
		bin("10.0.19041.0", "arm64"),
		bin("10.0.19041.0", "x64"),
		bin("10.0.19041.0", "x86"),
		bin("10.0.22621.0", "arm64"),
		bin("10.0.22621.0", "x64"),
		bin("10.0.22621.0", "x86"),
		bin("10.0.9600.0", "x64"),
	}

	assert.Equal(t, filepath.Dir(bin("10.0.22621.0", "x64")), pickWindowsSDKBin(matches, "amd64"))
	assert.Equal(t, filepath.Dir(bin("10.0.22621.0", "arm64")), pickWindowsSDKBin(matches, "arm64"))
	assert.Equal(t, filepath.Dir(bin("10.0.22621.0", "x86")), pickWindowsSDKBin(matches, "386"))

	// Only an older x86 toolset beside a newer arm64 one: x86 still runs on x64.
	assert.Equal(t, filepath.Dir(bin("10.0.19041.0", "x86")),
		pickWindowsSDKBin([]string{bin("10.0.22621.0", "arm64"), bin("10.0.19041.0", "x86")}, "amd64"))
}
