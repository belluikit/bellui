package dialog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

func TestFileDialog_WindowHosted_ShowHide(t *testing.T) {
	if !windowHostSupported {
		t.Skip("window hosting is unsupported on this build target")
	}
	test.NewTempApp(t)
	w := test.NewTempWindow(t, nil)
	w.Resize(geo.NewSize(600, 400))

	windowsBefore := len(bellui.CurrentApp().Driver().AllWindows())

	d := NewFileOpen(func(bellui.URIReadCloser, error) {}, w)
	d.SetWindowHosted(true)
	d.Show()

	require.NotNil(t, d.dialog.hostWin, "Show opens a host window")
	assert.Len(t, bellui.CurrentApp().Driver().AllWindows(), windowsBefore+1)
	assert.Equal(t, d.dialog.body, d.dialog.hostWin.Content(), "the dialog UI is the window content")

	require.NotNil(t, d.dialog.scrim, "the parent canvas is blocked by a scrim")
	assert.Equal(t, d.dialog.scrim, w.Canvas().Overlays().Top())
	assert.Equal(t, d.dialog.hostWin.Canvas(), d.dialog.canvas(),
		"internal popups anchor at the host window's canvas")

	d.Hide()
	assert.Nil(t, d.dialog.hostWin, "Hide closes the host window")
	assert.Nil(t, d.dialog.scrim)
	assert.Nil(t, w.Canvas().Overlays().Top(), "the scrim is removed with the window")
	assert.Len(t, bellui.CurrentApp().Driver().AllWindows(), windowsBefore)
}

func TestFileDialog_WindowHosted_CancelCallback(t *testing.T) {
	if !windowHostSupported {
		t.Skip("window hosting is unsupported on this build target")
	}
	test.NewTempApp(t)
	w := test.NewTempWindow(t, nil)

	calls := 0
	var gotReader bellui.URIReadCloser
	d := NewFileOpen(func(r bellui.URIReadCloser, _ error) { calls++; gotReader = r }, w)
	d.SetWindowHosted(true)
	d.Show()

	test.Tap(d.dialog.dismiss)

	assert.Equal(t, 1, calls, "cancelling runs the result callback once")
	assert.Nil(t, gotReader, "with a nil reader")
	assert.Nil(t, d.dialog.hostWin, "the host window is closed")
	assert.Nil(t, w.Canvas().Overlays().Top(), "the scrim is removed")
}

func TestFileDialog_WindowHosted_UserCloseDismisses(t *testing.T) {
	if !windowHostSupported {
		t.Skip("window hosting is unsupported on this build target")
	}
	test.NewTempApp(t)
	w := test.NewTempWindow(t, nil)

	calls := 0
	d := NewFileOpen(func(bellui.URIReadCloser, error) { calls++ }, w)
	d.SetWindowHosted(true)
	d.Show()

	// The host window being closed from outside the dialog API (window
	// manager, application code) reports a cancellation exactly once.
	d.dialog.hostWin.Close()

	assert.Equal(t, 1, calls)
	assert.Nil(t, d.dialog.hostWin)
	assert.Nil(t, w.Canvas().Overlays().Top(), "the scrim is removed on external close")
}

func TestFileDialog_WindowHosted_EscapeDismisses(t *testing.T) {
	if !windowHostSupported {
		t.Skip("window hosting is unsupported on this build target")
	}
	test.NewTempApp(t)
	w := test.NewTempWindow(t, nil)

	calls := 0
	d := NewFileOpen(func(bellui.URIReadCloser, error) { calls++ }, w)
	d.SetWindowHosted(true)
	d.Show()

	onKey := d.dialog.hostWin.Canvas().OnTypedKey()
	require.NotNil(t, onKey)
	onKey(&event.KeyEvent{Name: event.KeyEscape})

	assert.Equal(t, 1, calls, "Escape dismisses the dialog")
	assert.Nil(t, d.dialog.hostWin)
}

func TestFileDialog_WindowHosted_ReShow(t *testing.T) {
	if !windowHostSupported {
		t.Skip("window hosting is unsupported on this build target")
	}
	test.NewTempApp(t)
	w := test.NewTempWindow(t, nil)

	d := NewFileOpen(func(bellui.URIReadCloser, error) {}, w)
	d.SetWindowHosted(true)
	d.Show()
	first := d.dialog.hostWin
	d.Hide()

	d.Show()
	defer d.Hide()
	require.NotNil(t, d.dialog.hostWin, "a hidden dialog can be shown again")
	assert.NotSame(t, first, d.dialog.hostWin, "each show opens a fresh window")
	assert.Equal(t, d.dialog.scrim, w.Canvas().Overlays().Top())

	// A second Show while open focuses the existing window instead.
	second := d.dialog.hostWin
	d.Show()
	assert.Same(t, second, d.dialog.hostWin)
}

func TestFileDialog_WindowHosted_ResizeRoutesToWindow(t *testing.T) {
	if !windowHostSupported {
		t.Skip("window hosting is unsupported on this build target")
	}
	test.NewTempApp(t)
	w := test.NewTempWindow(t, nil)

	d := NewFileOpen(func(bellui.URIReadCloser, error) {}, w)
	d.SetWindowHosted(true)
	d.Resize(geo.NewSize(500, 400))
	d.Show()
	defer d.Hide()

	assert.Equal(t, geo.NewSize(500, 400), d.dialog.hostWin.Canvas().Size())

	d.Resize(geo.NewSize(550, 450))
	assert.Equal(t, geo.NewSize(550, 450), d.dialog.hostWin.Canvas().Size())
}

func TestFileDialog_WindowHosted_FolderOpen(t *testing.T) {
	if !windowHostSupported {
		t.Skip("window hosting is unsupported on this build target")
	}
	test.NewTempApp(t)
	w := test.NewTempWindow(t, nil)

	d := NewFolderOpen(func(bellui.ListableURI, error) {}, w)
	d.SetWindowHosted(true)
	d.Show()
	defer d.Hide()

	require.NotNil(t, d.dialog.hostWin, "a folder dialog also hosts in its own window")
	assert.Equal(t, d.dialog.body, d.dialog.hostWin.Content())
}
