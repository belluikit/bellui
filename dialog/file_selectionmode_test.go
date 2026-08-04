// Touch selection mode for the canvas file dialog (#259): long-pressing a
// file item enters a mode where plain taps toggle multi-select membership —
// the modifier-less counterpart to Ctrl/Cmd/Shift-click for touchscreens and
// picker-less mobile embeddings — and Escape/Back leaves the mode before it
// cancels the dialog. Same !wasm split as the other multi-select tests: these
// write real files and drive the canvas browser.
//go:build !wasm

package dialog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	intdriver "github.com/belluikit/bellui/internal/driver"
	intWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// visibleFileItems maps display id → the rendered *fileDialogItem for every
// cell the files panel currently realises, so tests can drive the widgets the
// way a finger does (test.LongPress / test.Tap) rather than the internal seam.
func visibleFileItems(t *testing.T, fd *fileDialog) map[int]*fileDialogItem {
	t.Helper()
	items := map[int]*fileDialogItem{}
	scroll := test.WidgetRenderer(fd.files).Objects()[0].(*intWidget.Scroll)
	for _, cell := range scroll.Content.(*bellui.Container).Objects {
		w, ok := cell.(core.Widget)
		if !ok {
			continue
		}
		for _, o := range test.TempWidgetRenderer(t, w).Objects() {
			if fi, ok := o.(*fileDialogItem); ok {
				items[fi.id] = fi
			}
		}
	}
	return items
}

func TestFileDialog_SelectionMode_LongPressEntersTapToggles(t *testing.T) {
	opened := false
	d := newLoadedMultiDialog(t, func(readers []bellui.URIReadCloser, err error) {
		opened = true
		for _, r := range readers {
			_ = r.Close()
		}
	})
	fd := d.dialog
	ids := fileItemIDs(t, fd)
	require.GreaterOrEqual(t, len(ids), 3)
	items := visibleFileItems(t, fd)
	require.NotNil(t, items[ids[0]])
	require.NotNil(t, items[ids[1]])

	// Long-press enters selection mode with the held file chosen.
	test.LongPress(items[ids[0]])
	assert.True(t, fd.selectionMode, "long-press on a file enters selection mode")
	require.Len(t, fd.multiSelected, 1)

	// Plain taps now toggle membership instead of replacing the selection.
	test.Tap(items[ids[1]])
	assert.Len(t, fd.multiSelected, 2, "a tap in selection mode adds to the set")
	test.Tap(items[ids[1]])
	assert.Len(t, fd.multiSelected, 1, "a second tap toggles the file back out")

	// Two rapid taps on one file must not fire the double-tap open shortcut
	// while the mode is active — the second tap just changed the set.
	test.Tap(items[ids[1]])
	test.Tap(items[ids[1]])
	assert.False(t, opened, "double-tap open is suspended in selection mode")

	// Deselecting the last file leaves the mode.
	test.Tap(items[ids[0]])
	assert.False(t, fd.selectionMode, "an emptied set ends selection mode")
	assert.Empty(t, fd.multiSelected)
}

func TestFileDialog_SelectionMode_EscapeExitsBeforeCancel(t *testing.T) {
	called := false
	var got []bellui.URIReadCloser
	d := newLoadedMultiDialog(t, func(readers []bellui.URIReadCloser, err error) {
		called, got = true, readers
	})
	fd := d.dialog
	ids := fileItemIDs(t, fd)
	require.NotEmpty(t, ids)
	fd.enterSelectionMode(ids[0])
	require.True(t, fd.selectionMode)

	c := fd.canvas()
	// The first Escape leaves selection mode and keeps the dialog up.
	assert.True(t, intdriver.HandleEscape(c))
	assert.False(t, fd.selectionMode)
	assert.Empty(t, fd.multiSelected)
	assert.False(t, called, "leaving selection mode must not cancel the dialog")

	// The second Escape cancels the dialog with the multi-select nil result.
	assert.True(t, intdriver.HandleEscape(c))
	assert.True(t, called)
	assert.Nil(t, got)
}

func TestFileDialog_SelectionMode_ComposesWithModifiers(t *testing.T) {
	d := newLoadedMultiDialog(t, func([]bellui.URIReadCloser, error) {})
	fd := d.dialog
	ids := fileItemIDs(t, fd)
	require.GreaterOrEqual(t, len(ids), 3)

	fd.enterSelectionMode(ids[0])
	// Shift keeps its range meaning over the mode's toggle default.
	fd.applyMultiTap(ids[2], event.KeyModifierShift)
	assert.Len(t, fd.multiSelected, 3, "Shift-tap range-selects from the long-press anchor")
	// Ctrl/Cmd toggle is indistinguishable from the mode's own toggle.
	fd.applyMultiTap(ids[1], event.KeyModifierControl)
	assert.Len(t, fd.multiSelected, 2)
	assert.True(t, fd.selectionMode, "modifier taps do not end the mode")
}

func TestFileDialog_SelectionMode_DirsAndSingleSelectInert(t *testing.T) {
	d := newLoadedMultiDialog(t, func([]bellui.URIReadCloser, error) {})
	fd := d.dialog
	files := fileItemIDs(t, fd)
	dirID := -1
	for id := 0; id < fd.displayCount(); id++ {
		isFile := false
		for _, f := range files {
			if f == id {
				isFile = true
				break
			}
		}
		if !isFile {
			dirID = id
			break
		}
	}
	require.GreaterOrEqual(t, dirID, 0, "the listing should include the parent entry")
	fd.enterSelectionMode(dirID)
	assert.False(t, fd.selectionMode, "directories navigate; they never enter selection mode")

	// A single-select dialog has no set to build: long-press is inert.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644))
	start, err := storage.ListerForURI(storage.NewFileURI(dir))
	require.NoError(t, err)
	w := test.NewTempWindow(t, widget.NewLabel("x"))
	s := NewFileOpen(func(bellui.URIReadCloser, error) {}, w)
	s.SetLocation(start)
	s.Show()
	sd := s.dialog
	for id := 0; id < sd.displayCount(); id++ {
		sd.enterSelectionMode(id)
	}
	assert.False(t, sd.selectionMode, "single-select dialogs never enter selection mode")
}
