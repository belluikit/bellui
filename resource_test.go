package bellui

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResource(t *testing.T) {
	name := "test"
	content := []byte{1, 2, 3, 4}

	res := NewStaticResource(name, content)
	assert.Equal(t, name, res.Name())
	assert.Equal(t, content, res.Content())
}

func TestLazyResource_Content(t *testing.T) {
	var calls atomic.Int32
	data := []byte("hello lazy")

	res := NewLazyResource("test.txt", func() ([]byte, error) {
		calls.Add(1)
		return data, nil
	})

	assert.Equal(t, "test.txt", res.Name())
	assert.False(t, res.Loaded())

	// First call should trigger loader
	got := res.Content()
	assert.Equal(t, data, got)
	assert.True(t, res.Loaded())
	assert.Equal(t, int32(1), calls.Load())

	// Second call should return cached data without calling loader
	got2 := res.Content()
	assert.Equal(t, data, got2)
	assert.Equal(t, int32(1), calls.Load())
}

func TestLazyResource_Release(t *testing.T) {
	var calls atomic.Int32
	data := []byte("release me")

	res := NewLazyResource("test.txt", func() ([]byte, error) {
		calls.Add(1)
		return data, nil
	})

	res.Content()
	assert.Equal(t, int32(1), calls.Load())
	assert.True(t, res.Loaded())

	res.Release()
	assert.False(t, res.Loaded())

	// Next Content() should re-load
	got := res.Content()
	assert.Equal(t, data, got)
	assert.Equal(t, int32(2), calls.Load())
}

func TestLazyResource_ContentError(t *testing.T) {
	var calls atomic.Int32

	res := NewLazyResource("bad.txt", func() ([]byte, error) {
		calls.Add(1)
		return nil, errors.New("load failed")
	})

	got := res.Content()
	assert.Nil(t, got)
	assert.False(t, res.Loaded())

	// Should retry on next call
	got = res.Content()
	assert.Nil(t, got)
	assert.Equal(t, int32(2), calls.Load())
}

func TestLazyResource_OpenWithOpener(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	content := []byte{0x89, 'P', 'N', 'G'}
	require.NoError(t, os.WriteFile(path, content, 0o644))

	res := NewLazyResourceFromPath(path)

	// Open should return a fresh reader without caching
	rc, err := res.Open()
	require.NoError(t, err)
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	rc.Close()
	assert.Equal(t, content, data)

	// Content should NOT have been cached by Open
	assert.False(t, res.Loaded())
}

func TestLazyResource_OpenFallback(t *testing.T) {
	data := []byte("fallback")
	res := NewLazyResource("test.txt", func() ([]byte, error) {
		return data, nil
	})

	// Open without opener should fall back to Content()
	rc, err := res.Open()
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	rc.Close()
	assert.Equal(t, data, got)
	assert.True(t, res.Loaded())
}

func TestLazyResourceFromPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "image.png")
	content := []byte{1, 2, 3, 4, 5}
	require.NoError(t, os.WriteFile(path, content, 0o644))

	res := NewLazyResourceFromPath(path)
	assert.Equal(t, "image.png", res.Name())
	assert.Equal(t, content, res.Content())
	assert.True(t, res.Loaded())
}

func TestLazyResource_ImplementsStreamingResource(t *testing.T) {
	res := NewLazyResource("test.txt", func() ([]byte, error) {
		return []byte("data"), nil
	})
	var _ StreamingResource = res
}

func TestLazyResource_ConcurrentAccess(t *testing.T) {
	var calls atomic.Int32
	data := []byte("concurrent")

	res := NewLazyResource("test.txt", func() ([]byte, error) {
		calls.Add(1)
		return data, nil
	})

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := res.Content()
			assert.Equal(t, data, got)
		}()
	}
	wg.Wait()

	// Loader should have been called exactly once
	assert.Equal(t, int32(1), calls.Load())
}
