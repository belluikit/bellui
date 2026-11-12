package util

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildWindowsExe cross-builds a pure-Go Windows executable, which imports
// kernel32.dll like every Windows program.
func buildWindowsExe(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/w\n\ngo 1.25\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() { println(\"w\") }\n"), 0o644))
	exe := filepath.Join(dir, "w.exe")
	cmd := exec.Command("go", "build", "-o", exe, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0", "GOWORK=off", "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return exe
}

func TestSharedLibDirs_SelfBinary(t *testing.T) {
	// The test binary links no goffmpeg-libs module: nothing to bundle, and
	// no error.
	exe, err := os.Executable()
	require.NoError(t, err)
	dirs, err := SharedLibDirs(exe)
	require.NoError(t, err)
	assert.Empty(t, dirs)
}

func TestModuleDir_MissingFromTheCache(t *testing.T) {
	_, err := ModuleDir("github.com/belluikit/goffmpeg-libs/Nowhere_arm64", "v0.0.0-missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "go mod download")
	assert.Contains(t, err.Error(), "!nowhere_arm64@v0.0.0-missing", "the cache path is escaped")
}

// pe.File.ImportedLibraries returns nothing, so the DLL names must come from
// the imported symbols (#1101).
func TestImportedLibs_PE(t *testing.T) {
	libs, err := ImportedLibs(buildWindowsExe(t))
	require.NoError(t, err)
	assert.Contains(t, strings.ToLower(strings.Join(libs, " ")), "kernel32.dll")
}

// An iOS binary names a framework's library by its path inside the
// framework (@rpath/libavutil-goffmpeg.framework/libavutil-goffmpeg); the
// lookup finds it there, and the library knows its framework (#1122).
func TestSharedLibs_FindsFrameworks(t *testing.T) {
	shared := t.TempDir()
	fw := filepath.Join(shared, "libavutil-goffmpeg.framework")
	require.NoError(t, os.MkdirAll(fw, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(fw, "libavutil-goffmpeg"), []byte("x"), 0o644))

	name := "libavutil-goffmpeg.framework/libavutil-goffmpeg"
	assert.Equal(t, filepath.Join(fw, "libavutil-goffmpeg"), findLib([]string{shared}, name))
	assert.Empty(t, findLib([]string{shared}, "libavutil-goffmpeg.framework"), "a directory is not a library")
	assert.Empty(t, findLib([]string{shared}, "Other.framework/Other"))

	assert.Equal(t, "libavutil-goffmpeg.framework", SharedLib{Name: name}.Framework())
	assert.Empty(t, SharedLib{Name: "libavutil-goffmpeg.so"}.Framework())
}
