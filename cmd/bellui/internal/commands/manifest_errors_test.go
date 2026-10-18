package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A manifest cut off before its closing brace, as an editor crash leaves it.
const brokenManifest = "details {\n  id = \"com.example.probe\"\n  version = \"2.0.0\"\n"

func appDir(t *testing.T, manifest string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	if manifest != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.pxf"), []byte(manifest), 0o644))
	}
	return dir
}

func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// bellui build used to treat a manifest that does not parse as no manifest at
// all, and built the app without its ID, name or version (#360).
func TestCreateMetadataInitFileRejectsBrokenManifest(t *testing.T) {
	dir := appDir(t, brokenManifest)

	_, err := createMetadataInitFile(dir, &appData{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manifest.pxf")
	assert.ElementsMatch(t, []string{"main.go", "manifest.pxf"}, dirEntries(t, dir), "nothing may be written")
}

func TestCreateMetadataInitFileWithoutManifest(t *testing.T) {
	dir := appDir(t, "")

	cleanup, err := createMetadataInitFile(dir, &appData{})
	require.NoError(t, err)
	cleanup()
}

// bellui package used to write a bundle with a placeholder identity before a
// broken manifest stopped it, with an error that did not name the file (#360).
func TestPackagerRejectsBrokenManifest(t *testing.T) {
	dir := appDir(t, brokenManifest)
	t.Chdir(dir) // validate changes directory; t.Chdir restores it

	p := &Packager{appData: &appData{}, srcDir: dir, os: "darwin"}
	err := p.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manifest.pxf")
	assert.ElementsMatch(t, []string{"main.go", "manifest.pxf"}, dirEntries(t, dir), "nothing may be written")
	assert.NoDirExists(t, p.tempDir, "the temporary directory must be removed")
}

func TestPackagerWithoutManifest(t *testing.T) {
	dir := appDir(t, "")
	t.Chdir(dir)

	p := &Packager{appData: &appData{}, srcDir: dir, os: "darwin"}
	require.NoError(t, p.validate())
	t.Cleanup(func() { _ = os.RemoveAll(p.tempDir) })
}

// The build itself must stop, not only the init file: bellui build used to
// log the failed injection and build the app without its metadata (#360).
// The mock runner fails the test if go build is run.
func TestBuildStopsOnBrokenManifest(t *testing.T) {
	dir := appDir(t, brokenManifest)
	runs := &testCommandRuns{t: t, runs: []mockRunner{{
		expectedValue: expectedValue{args: []string{"mod", "edit", "-json"}, dir: dir},
		mockReturn:    mockReturn{ret: []byte(`{ "Module": { "Path": "github.com/belluikit/bellui"} }`)},
	}}}

	b := &Builder{appData: &appData{}, os: "wasm", srcdir: dir, runner: runs}
	err := b.build()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manifest.pxf")
	runs.verifyExpectation()
	assert.ElementsMatch(t, []string{"main.go", "manifest.pxf"}, dirEntries(t, dir), "nothing may be written")
}
