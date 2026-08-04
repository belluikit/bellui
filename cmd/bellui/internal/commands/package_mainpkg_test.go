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

	require.NoError(t, os.WriteFile(filepath.Join(dir, "main_windows.go"), []byte("//go:build windows\n\npackage main\n"), 0o644))
	name, ok = mainPackageIn(dir)
	assert.True(t, ok, "a build-tagged main for another OS still counts")
	assert.Equal(t, "main", name)
}
