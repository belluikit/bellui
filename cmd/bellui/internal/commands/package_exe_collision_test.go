package commands

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGoBuild stands in for the go tool: it answers `go mod edit -json` as an
// app that requires bellui, and `go build -o X` by writing a file at X, so
// packaging runs for real everywhere except the compiler.
type fakeGoBuild struct {
	t      *testing.T
	built  []string
	srcDir string
}

func (f *fakeGoBuild) setDir(dir string) { assert.Equal(f.t, f.srcDir, dir) }
func (f *fakeGoBuild) setEnv([]string)   {}

func (f *fakeGoBuild) runOutput(args ...string) ([]byte, error) {
	if len(args) > 0 && args[0] == "build" {
		for i, a := range args {
			if a == "-o" && i+1 < len(args) {
				f.built = append(f.built, args[i+1])
				return nil, os.WriteFile(args[i+1], []byte("binary"), 0o755)
			}
		}
		f.t.Fatalf("go build without -o: %v", args)
	}
	return []byte(`{"Module":{"Path":"example.com/app"},"Require":[{"Path":"github.com/belluikit/bellui"}]}`), nil
}

// packageModuleNamed packages, for osys, a main package whose module path ends
// in name, from that package's own directory as `bellui package` and `bellui
// serve` do, and returns the directory.
func packageModuleNamed(t *testing.T, name, osys string, runs int) (string, *fakeGoBuild) {
	t.Helper()
	prevUtil := util
	util = defaultUtil{}
	t.Cleanup(func() { util = prevUtil })

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module example.com/apps/"+name+"\n\ngo 1.25\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	t.Chdir(dir) // validate changes into the source directory; restore it afterwards

	fake := &fakeGoBuild{t: t, srcDir: dir}
	for i := 0; i < runs; i++ {
		p := &Packager{appData: &appData{}, os: osys}
		require.NoError(t, p.validate())
		require.NoError(t, p.doPackage(fake), "packaging run %d", i+1)
	}
	return dir, fake
}

// A module whose last element is wasm used to build its binary to <src>/wasm,
// the very path the web package then needed as its output folder, so both
// `bellui package -os wasm` and `bellui serve` failed with "not a directory"
// (#683). The second run is the serve-again case: the wasm/ folder from the
// first run must not be mistaken for a prebuilt executable.
func TestPackageWasm_ModuleNamedWasm(t *testing.T) {
	for _, osys := range []string{"wasm", "web"} {
		t.Run(osys, func(t *testing.T) {
			dir, fake := packageModuleNamed(t, "wasm", osys, 2)

			require.Len(t, fake.built, 2, "each run builds")
			for _, exe := range fake.built {
				assert.NotEqual(t, dir, filepath.Dir(exe), "the intermediate binary must not be built into the source directory")
			}

			out := filepath.Join(dir, "wasm")
			for _, f := range []string{"index.html", "wasm.wasm", "wasm_exec.js", "icon.png"} {
				info, err := os.Stat(filepath.Join(out, f))
				if assert.NoError(t, err, "the web package must hold %s", f) {
					assert.False(t, info.IsDir())
				}
			}
			assertSourceDirHolds(t, dir, "go.mod", "main.go", "wasm")
		})
	}
}

// The same collision for the UNIX staging folder: a module named tmp-pkg built
// <src>/tmp-pkg, where packageUNIX stages the archive.
func TestPackageUNIX_ModuleNamedTmpPkg(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("packageUNIX archives with tar -J, which Windows' tar does not provide")
	}
	dir, fake := packageModuleNamed(t, "tmp-pkg", "linux", 1)

	require.Len(t, fake.built, 1)
	assert.NotEqual(t, dir, filepath.Dir(fake.built[0]))
	info, err := os.Stat(filepath.Join(dir, "tmp-pkg.tar.xz"))
	require.NoError(t, err, "the archive is written to the output directory as before")
	assert.False(t, info.IsDir())
}

// Windows is the one target whose built executable is the product, so it stays
// in the source directory; every other target only copies it into a package.
func TestDefaultExePath(t *testing.T) {
	p := &Packager{srcDir: filepath.Join("src", "app"), tempDir: filepath.Join("tmp", "bellui-package-1")}
	for _, osys := range []string{"darwin", "linux", "freebsd", "openbsd", "netbsd", "wasm", "web", "android", "ios"} {
		p.os = osys
		assert.Equal(t, filepath.Join(p.tempDir, "wasm"), p.defaultExePath("wasm"), osys)
	}
	p.os = "windows"
	assert.Equal(t, filepath.Join(p.srcDir, "wasm.exe"), p.defaultExePath("wasm.exe"))
}

// assertSourceDirHolds checks packaging left nothing in dir but want: no
// intermediate binary and no injected metadata file.
func assertSourceDirHolds(t *testing.T, dir string, want ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	assert.ElementsMatch(t, want, got)
}
