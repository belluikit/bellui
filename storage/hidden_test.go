// IsHidden is documented always-false on mobile/wasm — there is no
// dot-prefix or attribute convention to detect in a browser — so these
// platform-convention assertions are !wasm (#79 Phase B).
//go:build !wasm

package storage_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/storage"
)

func TestIsHidden(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses the hidden file attribute, not the dot prefix")
	}

	dir := t.TempDir()
	assert.True(t, storage.IsHidden(storage.NewFileURI(filepath.Join(dir, ".hidden"))))
	assert.False(t, storage.IsHidden(storage.NewFileURI(filepath.Join(dir, "visible.txt"))))

	remote, err := storage.ParseURI("https://example.com/.hidden")
	assert.NoError(t, err)
	assert.False(t, storage.IsHidden(remote), "non-file URIs are never hidden")
}
