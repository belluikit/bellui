package commands

import (
	"bytes"
	"debug/elf"
	"debug/macho"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
)

func TestSharedLibDirs_SelfBinary(t *testing.T) {
	// The test binary links no goffmpeg-libs module: nothing to bundle, and
	// no error.
	exe, err := os.Executable()
	require.NoError(t, err)
	dirs, err := sharedLibDirs(exe)
	require.NoError(t, err)
	assert.Empty(t, dirs)
}

func TestModuleDir_MissingFromTheCache(t *testing.T) {
	_, err := moduleDir("github.com/belluikit/goffmpeg-libs/Nowhere_arm64", "v0.0.0-missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "go mod download")
	assert.Contains(t, err.Error(), "!nowhere_arm64@v0.0.0-missing", "the cache path is escaped")
}

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

// pe.File.ImportedLibraries returns nothing, so the DLL names must come from
// the imported symbols (#1101).
func TestImportedLibs_PE(t *testing.T) {
	libs, err := importedLibs(buildWindowsExe(t))
	require.NoError(t, err)
	assert.Contains(t, strings.ToLower(strings.Join(libs, " ")), "kernel32.dll")
}

func TestBundleSharedLibs_CopiesWhatTheBinaryLoads(t *testing.T) {
	prevUtil := util
	util = defaultUtil{}
	t.Cleanup(func() { util = prevUtil })
	exe := buildWindowsExe(t)
	exeData, err := os.ReadFile(exe)
	require.NoError(t, err)

	// A "shared/" directory holding a DLL the executable imports (named in
	// another case, as Windows allows) and one it doesn't. The imported one is
	// a PE file itself, so its own imports are read in turn.
	shared := filepath.Join(t.TempDir(), "shared")
	require.NoError(t, os.MkdirAll(shared, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(shared, "KERNEL32.DLL"), exeData, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(shared, "unused-goffmpeg-1.dll"), []byte("x"), 0o644))

	dest := filepath.Join(t.TempDir(), "out")
	copied, err := bundleSharedLibs(exe, []string{shared}, dest)
	require.NoError(t, err)
	require.Len(t, copied, 1)
	assert.True(t, strings.EqualFold(copied[0], "kernel32.dll"), copied[0])
	assert.FileExists(t, filepath.Join(dest, copied[0]))
	assert.NoFileExists(t, filepath.Join(dest, "unused-goffmpeg-1.dll"))
}

// buildCgoWithRpath builds a cgo program linked with rpaths, and returns it.
func buildCgoWithRpath(t *testing.T, rpaths ...string) string {
	t.Helper()
	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("needs a C compiler for cgo")
	}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/r\n\ngo 1.25\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\n// int one(void) { return 1; }\nimport \"C\"\n\nfunc main() { println(C.one()) }\n"), 0o644))
	exe := filepath.Join(dir, "r")
	var flags []string
	for _, r := range rpaths {
		flags = append(flags, "-Wl,-rpath,"+r)
	}
	cmd := exec.Command("go", "build", "-o", exe, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1", "GOWORK=off", "GOFLAGS=", "CGO_LDFLAGS="+strings.Join(flags, " "))
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return exe
}

// fakeModuleSet is a module-cache path of a goffmpeg-libs set directory.
func fakeModuleSet(t *testing.T) string {
	return filepath.Join(t.TempDir(), "pkg", "mod", "github.com", "belluikit", "goffmpeg-libs", "linux_amd64@v0.6.0", "shared")
}

func TestRelinkELF(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("links an ELF program")
	}
	set := fakeModuleSet(t)
	// Both sets' rpaths go, whichever one the build linked.
	exe := buildCgoWithRpath(t, set, filepath.Join(filepath.Dir(set), "shared-audio"), "/opt/keep")

	require.NoError(t, relinkELF(exe, []string{set}, "$ORIGIN/../lib/com.example.app"))

	f, err := elf.Open(exe)
	require.NoError(t, err)
	runpath, _ := f.DynString(elf.DT_RUNPATH)
	rpath, _ := f.DynString(elf.DT_RPATH)
	f.Close()
	assert.Equal(t, []string{"/opt/keep:$ORIGIN/../lib/com.example.app"}, append(runpath, rpath...))
	out, err := exec.Command(exe).CombinedOutput()
	require.NoError(t, err, "the relinked program still runs: %s", out)

	// A replacement longer than the rpath it replaces doesn't fit.
	short := buildCgoWithRpath(t, "/m/goffmpeg-libs/x@v/shared")
	err = relinkELF(short, []string{"/m/goffmpeg-libs/x@v/shared"}, "$ORIGIN/../lib/"+strings.Repeat("a", 64))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not fit")
}

func TestRelinkDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("links a Mach-O program and needs install_name_tool and codesign")
	}
	set := fakeModuleSet(t)
	exe := buildCgoWithRpath(t, set, filepath.Join(filepath.Dir(set), "shared-audio"), "/opt/keep")

	require.NoError(t, relinkDarwin(exe, []string{set}, "@executable_path/../Frameworks"))

	f, err := macho.Open(exe)
	require.NoError(t, err)
	var rpaths []string
	for _, l := range f.Loads {
		if r, ok := l.(*macho.Rpath); ok {
			rpaths = append(rpaths, r.Path)
		}
	}
	f.Close()
	assert.Equal(t, []string{"/opt/keep", "@executable_path/../Frameworks"}, rpaths)
	// arm64 runs no code whose signature the relink broke.
	out, err := exec.Command(exe).CombinedOutput()
	require.NoError(t, err, "the relinked program still runs: %s", out)
	out, err = exec.Command("codesign", "--verify", exe).CombinedOutput()
	require.NoError(t, err, string(out))
}

func TestMakefileTemplate_LibBlocks(t *testing.T) {
	render := func(bundle bool) string {
		var buf bytes.Buffer
		require.NoError(t, templates.MakefileUNIX.Execute(&buf, unixData{
			Name: "App", AppID: "com.example.app", Exec: "app", Icon: "app.png",
			BundleLibs: bundle,
		}))
		return buf.String()
	}

	with := render(true)
	assert.Contains(t, with, "cp -r usr/lib/$(AppID)/. $(DESTDIR)$(PREFIX)/lib/$(AppID)/")
	assert.Contains(t, with, "cp -r usr/lib/$(AppID)/. $(DESTDIR)$(HOME)/.local/lib/$(AppID)/")
	assert.Contains(t, with, "-rm -r $(DESTDIR)$(PREFIX)/lib/$(AppID)")
	assert.Contains(t, with, "-rm -r $(DESTDIR)$(HOME)/.local/lib/$(AppID)")

	assert.NotContains(t, render(false), "lib/$(AppID)")
}
