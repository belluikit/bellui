package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMainPackageIn(t *testing.T) {
	dir := t.TempDir()

	name, ok := mainPackageIn(dir)
	assert.False(t, ok, "an empty directory has no main package")
	assert.Equal(t, "", name)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "lib.go"), []byte("package notmain\n"), 0o644))
	name, ok = mainPackageIn(dir)
	assert.False(t, ok, "a library package is not a main package")
	assert.Equal(t, "notmain", name, "the found package name feeds the error message")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "main_test.go"), []byte("package main\n"), 0o644))
	_, ok = mainPackageIn(dir)
	assert.False(t, ok, "test files do not make a main package")

	sub := filepath.Join(dir, "app")
	require.NoError(t, os.Mkdir(sub, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "main.go"), []byte("package main\n"), 0o644))
	_, ok = mainPackageIn(dir)
	assert.False(t, ok, "a main package in a subdirectory does not count — package builds the directory's own package (#248)")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "_ignored.go"), []byte("package main\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".hidden.go"), []byte("package main\n"), 0o644))
	name, ok = mainPackageIn(dir)
	assert.False(t, ok, "files the go tool ignores (_ and . prefixes) do not make a main package")
	assert.Equal(t, "notmain", name, "ignored files do not feed the error message either")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "main_windows.go"), []byte("//go:build windows\n\npackage main\n"), 0o644))
	name, ok = mainPackageIn(dir)
	assert.True(t, ok, "a build-tagged main for another OS still counts")
	assert.Equal(t, "main", name)
}

func TestValidateMainPackage(t *testing.T) {
	prevUtil := util
	util = defaultUtil{}
	t.Cleanup(func() { util = prevUtil })

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "lib.go"), []byte("package notmain\n"), 0o644))

	p := &Packager{appData: &appData{}, srcDir: dir, os: "linux"}
	err := p.validateMainPackage()
	require.Error(t, err, "building a non-main source directory must fail fast")
	assert.Contains(t, err.Error(), `is package "notmain", not a main package`)

	// A prebuilt --executable is wrapped, never built, so no metadata file is
	// injected and the source package is irrelevant.
	p.exe = filepath.Join(dir, "prebuilt")
	require.NoError(t, os.WriteFile(p.exe, []byte("binary"), 0o755))
	assert.NoError(t, p.validateMainPackage(), "wrapping a prebuilt executable does not build the source directory")

	// ...but mobile targets always build through gomobile (--executable is
	// ignored there), so they are always checked.
	p.os = "android"
	assert.Error(t, p.validateMainPackage(), "mobile always builds, so the exemption does not apply")

	// A directory whose only Go code lives in subdirectories (or in files the
	// go tool ignores) gets its own message instead of `package ""`.
	root := t.TempDir()
	sub := filepath.Join(root, "app")
	require.NoError(t, os.Mkdir(sub, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "main.go"), []byte("package main\n"), 0o644))
	p = &Packager{appData: &appData{}, srcDir: root, os: "linux"}
	err = p.validateMainPackage()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has no buildable Go files of its own")

	require.NoError(t, os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644))
	assert.NoError(t, p.validateMainPackage(), "a main package passes")
}
