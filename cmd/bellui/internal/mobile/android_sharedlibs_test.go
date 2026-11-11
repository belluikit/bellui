package mobile

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The libraries an app library loads from its Go modules land in its own
// lib/<abi>/ directory, where Android's linker looks for them, and are named
// there by APK path (#1122). A Windows executable stands in for the library:
// it is the one binary every host can build that names a library it loads
// (kernel32.dll), and the walk reads every format alike.
func TestCopyAndroidSharedLibs(t *testing.T) {
	prevTmp := tmpdir
	tmpdir = t.TempDir()
	t.Cleanup(func() { tmpdir = prevTmp })

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/w\n\ngo 1.25\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	lib := filepath.Join(dir, "libapp.so")
	cmd := exec.Command("go", "build", "-o", lib, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0", "GOWORK=off", "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	shared := filepath.Join(t.TempDir(), "shared")
	require.NoError(t, os.MkdirAll(shared, 0o755))
	exe, err := os.ReadFile(lib)
	require.NoError(t, err)
	// A binary itself, since what it loads is read in turn.
	require.NoError(t, os.WriteFile(filepath.Join(shared, "kernel32.dll"), exe, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(shared, "libunused-goffmpeg.so"), []byte("x"), 0o644))

	files, err := copyAndroidSharedLibs(lib, []string{shared}, "lib/arm64-v8a")
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.True(t, strings.EqualFold(files[0], "lib/arm64-v8a/kernel32.dll"), files[0])
	assert.FileExists(t, filepath.Join(tmpdir, "lib", "arm64-v8a", filepath.Base(files[0])))
	assert.NoFileExists(t, filepath.Join(tmpdir, "lib", "arm64-v8a", "libunused-goffmpeg.so"))
}
