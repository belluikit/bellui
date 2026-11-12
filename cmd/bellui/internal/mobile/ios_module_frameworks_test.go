package mobile

import (
	"debug/macho"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A binary that loads goffmpeg's frameworks gains the rpath they are found
// by in the app, once however often it is added (#1122).
func TestAddIOSRpath(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("links a Mach-O program and needs install_name_tool")
	}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/r\n\ngo 1.25\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\n// int one(void) { return 1; }\nimport \"C\"\n\nfunc main() { println(C.one()) }\n"), 0o644))
	bin := filepath.Join(dir, "r")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1", "GOWORK=off", "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	rpaths := func() []string {
		f, err := macho.Open(bin)
		require.NoError(t, err)
		defer f.Close()
		var out []string
		for _, l := range f.Loads {
			if r, ok := l.(*macho.Rpath); ok {
				out = append(out, r.Path)
			}
		}
		return out
	}
	require.NotContains(t, rpaths(), iosEmbeddedFrameworks)
	require.NoError(t, addIOSRpath(bin))
	require.NoError(t, addIOSRpath(bin))
	assert.Equal(t, []string{iosEmbeddedFrameworks}, rpaths())
}

// The binaries of a test program link no goffmpeg-libs module: nothing is
// embedded, and the embeds from ios/Frameworks stay as they were.
func TestGoModuleFrameworks_None(t *testing.T) {
	exe, err := os.Executable()
	require.NoError(t, err)
	embed := []iosFramework{{Name: "Foo", Path: "/app/ios/Frameworks/Foo.framework", Dynamic: true}}
	got, err := goModuleFrameworks(embed, []string{exe})
	require.NoError(t, err)
	assert.Equal(t, embed, got)
}
