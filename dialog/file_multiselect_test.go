// These tests write real files (os.WriteFile into t.TempDir, listed back via
// storage.ListerForURI on file:// URIs) and drive the canvas file browser —
// both the !wasm halves of their seams; the browser has neither (#77's split
// convention, #79).
//go:build !wasm

package dialog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// fileItemIDs returns the display ids (and names) of the selectable files in
// the loaded directory, skipping directories and the parent entry.
func fileItemIDs(t *testing.T, fd *fileDialog) []int {
	t.Helper()
	var ids []int
	for id := 0; id < fd.displayCount(); id++ {
		u, ok := fd.getDisplayItem(id)
		if !ok {
			continue
		}
		if listable, _ := storage.CanList(u); listable {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func newLoadedMultiDialog(t *testing.T, cb func([]bellui.URIReadCloser, error)) *FileDialog {
	t.Helper()
	dir := t.TempDir()
	for _, n := range []string{"a.txt", "b.txt", "c.txt"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644))
	}
	start, err := storage.ListerForURI(storage.NewFileURI(dir))
	require.NoError(t, err)

	w := test.NewTempWindow(t, widget.NewLabel("Content"))
	d := NewFileOpenMultiple(cb, w)
	d.SetLocation(start)
	d.Show()
	return d
}

func TestFileDialog_MultiSelect_ToggleAndReturn(t *testing.T) {
	var got []bellui.URIReadCloser
	var gotErr error
	called := false
	d := newLoadedMultiDialog(t, func(readers []bellui.URIReadCloser, err error) {
		called, got, gotErr = true, readers, err
	})
	fd := d.dialog
	assert.True(t, d.multiSelect)

	ids := fileItemIDs(t, fd)
	require.GreaterOrEqual(t, len(ids), 3)

	// Plain tap selects exactly one.
	fd.applyMultiTap(ids[0], 0)
	require.Len(t, fd.multiSelected, 1)

	// Ctrl-tap adds a second.
	fd.applyMultiTap(ids[1], event.KeyModifierControl)
	require.Len(t, fd.multiSelected, 2)

	// Ctrl-tap an already-selected item toggles it off.
	fd.applyMultiTap(ids[0], event.KeyModifierControl)
	require.Len(t, fd.multiSelected, 1)

	// Re-add and confirm OK returns every chosen reader.
	fd.applyMultiTap(ids[0], event.KeyModifierControl)
	require.Len(t, fd.multiSelected, 2)

	fd.open.OnTapped()
	require.True(t, called)
	require.NoError(t, gotErr)
	require.Len(t, got, 2)
	for _, r := range got {
		require.NotNil(t, r)
		_ = r.Close()
	}
}

func TestFileDialog_MultiSelect_ShiftRange(t *testing.T) {
	d := newLoadedMultiDialog(t, func([]bellui.URIReadCloser, error) {})
	fd := d.dialog
	ids := fileItemIDs(t, fd)
	require.GreaterOrEqual(t, len(ids), 3)

	// Anchor on the first file, Shift-tap the third → the whole contiguous run.
	fd.applyMultiTap(ids[0], 0)
	fd.applyMultiTap(ids[2], event.KeyModifierShift)
	assert.Len(t, fd.multiSelected, 3)
}

func TestFileDialog_SetMultiSelect_SaveIgnored(t *testing.T) {
	w := test.NewTempWindow(t, widget.NewLabel("x"))
	d := NewFileSave(nil, w)
	d.SetMultiSelect(true)
	assert.False(t, d.multiSelect, "save dialogs do not support multi-select")
}

func TestNewFileOpenMultiple_SetsMode(t *testing.T) {
	w := test.NewTempWindow(t, widget.NewLabel("x"))
	d := NewFileOpenMultiple(func([]bellui.URIReadCloser, error) {}, w)
	assert.True(t, d.multiSelect)
	assert.False(t, d.save)
}
