package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/storage"
)

func TestNewAssetBundle(t *testing.T) {
	dir := t.TempDir()
	b, err := storage.NewAssetBundle(dir)
	require.NoError(t, err)
	assert.NotNil(t, b)
}

func TestNewAssetBundle_NotExist(t *testing.T) {
	_, err := storage.NewAssetBundle("/nonexistent/path/12345")
	assert.Error(t, err)
}

func TestNewAssetBundle_NotDir(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(f, []byte("hi"), 0o644))

	_, err := storage.NewAssetBundle(f)
	assert.Error(t, err)
}

func TestAssetBundle_Resource(t *testing.T) {
	dir := t.TempDir()
	content := []byte{1, 2, 3, 4}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "icon.png"), content, 0o644))

	b, err := storage.NewAssetBundle(dir)
	require.NoError(t, err)

	res := b.Resource("icon.png")
	require.NotNil(t, res)
	assert.Equal(t, "icon.png", res.Name())
	assert.Equal(t, content, res.Content())
}

func TestAssetBundle_Resource_NotExist(t *testing.T) {
	dir := t.TempDir()
	b, err := storage.NewAssetBundle(dir)
	require.NoError(t, err)

	res := b.Resource("missing.png")
	assert.Nil(t, res)
}

func TestAssetBundle_Resource_Subdirectory(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "icons")
	require.NoError(t, os.MkdirAll(subdir, 0o755))
	content := []byte("svg content")
	require.NoError(t, os.WriteFile(filepath.Join(subdir, "cancel.svg"), content, 0o644))

	b, err := storage.NewAssetBundle(dir)
	require.NoError(t, err)

	res := b.Resource("icons/cancel.svg")
	require.NotNil(t, res)
	assert.Equal(t, "cancel.svg", res.Name())
	assert.Equal(t, content, res.Content())
}

func TestAssetBundle_Resource_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "icon.png"), []byte{1}, 0o644))

	// Create a file outside the bundle root that an attacker might target.
	outside := filepath.Join(filepath.Dir(dir), "secret.txt")
	require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o644))

	b, err := storage.NewAssetBundle(dir)
	require.NoError(t, err)

	// All traversal variants must return nil.
	assert.Nil(t, b.Resource("../secret.txt"), "single-level traversal")
	assert.Nil(t, b.Resource("../../secret.txt"), "multi-level traversal")
	assert.Nil(t, b.Resource("sub/../../secret.txt"), "traversal via subdirectory")

	// Valid resource still works.
	assert.NotNil(t, b.Resource("icon.png"))
}

func TestAssetBundle_Resource_Cached(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "icon.png"), []byte{1}, 0o644))

	b, err := storage.NewAssetBundle(dir)
	require.NoError(t, err)

	res1 := b.Resource("icon.png")
	res2 := b.Resource("icon.png")
	assert.Same(t, res1, res2)
}

func TestAssetBundle_MustResource(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "icon.png"), []byte{1}, 0o644))

	b, err := storage.NewAssetBundle(dir)
	require.NoError(t, err)

	res := b.MustResource("icon.png")
	assert.NotNil(t, res)
}

func TestAssetBundle_MustResource_Panics(t *testing.T) {
	dir := t.TempDir()
	b, err := storage.NewAssetBundle(dir)
	require.NoError(t, err)

	assert.Panics(t, func() {
		b.MustResource("missing.png")
	})
}

func TestAssetBundle_List(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.png"), []byte{1}, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.svg"), []byte{2}, 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0o755))

	b, err := storage.NewAssetBundle(dir)
	require.NoError(t, err)

	names, err := b.List()
	require.NoError(t, err)
	assert.Contains(t, names, "a.png")
	assert.Contains(t, names, "b.svg")
	assert.NotContains(t, names, "subdir")
}

func TestAssetBundle_ReleaseAll(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "icon.png"), []byte{1, 2}, 0o644))

	b, err := storage.NewAssetBundle(dir)
	require.NoError(t, err)

	res := b.Resource("icon.png")
	require.NotNil(t, res)
	_ = res.Content() // force load

	b.ReleaseAll()

	// Content should still work (re-loads)
	assert.Equal(t, []byte{1, 2}, res.Content())
}
