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

func TestDialog_WindowHosted_ShowHide(t *testing.T) {
	if !windowHostSupported {
		t.Skip("window hosting is unsupported on this build target")
	}
	test.NewTempApp(t)
	w := test.NewTempWindow(t, nil)
	w.Resize(geo.NewSize(600, 400))

	windowsBefore := len(bellui.CurrentApp().Driver().AllWindows())

	d := NewInformation("Title", "message", w)
	d.SetWindowHosted(true)
	d.Show()

	dlg := d.(*dialog)
	require.NotNil(t, dlg.hostWin, "Show opens a host window")
	assert.Len(t, bellui.CurrentApp().Driver().AllWindows(), windowsBefore+1)
	assert.Equal(t, "Title", dlg.hostWin.Title())
	assert.Equal(t, dlg.body, dlg.hostWin.Content(), "the dialog body is the window content")

	require.NotNil(t, dlg.scrim, "the parent canvas is blocked by a scrim")
	assert.Equal(t, dlg.scrim, w.Canvas().Overlays().Top())

	d.Hide()
	assert.Nil(t, dlg.hostWin, "Hide closes the host window")
	assert.Nil(t, dlg.scrim)
	assert.Nil(t, w.Canvas().Overlays().Top(), "the scrim is removed with the window")
	assert.Len(t, bellui.CurrentApp().Driver().AllWindows(), windowsBefore)
}

func TestDialog_WindowHosted_ConfirmCallback(t *testing.T) {
	if !windowHostSupported {
		t.Skip("window hosting is unsupported on this build target")
	}
	test.NewTempApp(t)
	w := test.NewTempWindow(t, nil)

	responses := []bool{}
	d := NewConfirm("Confirm", "Proceed?", func(ok bool) { responses = append(responses, ok) }, w)
	d.SetWindowHosted(true)
	d.Show()

	test.Tap(d.confirm)

	assert.Equal(t, []bool{true}, responses, "confirming runs the callback once, with true")
	assert.Nil(t, d.dialog.hostWin)
	assert.Nil(t, w.Canvas().Overlays().Top())
}

func TestDialog_WindowHosted_UserCloseDismisses(t *testing.T) {
	if !windowHostSupported {
		t.Skip("window hosting is unsupported on this build target")
	}
	test.NewTempApp(t)
	w := test.NewTempWindow(t, nil)

	responses := []bool{}
	d := NewConfirm("Confirm", "Proceed?", func(ok bool) { responses = append(responses, ok) }, w)
	d.SetWindowHosted(true)
	d.Show()

	// The host window being closed from outside the dialog API (window
	// manager, application code) reports a dismissal exactly once.
	d.dialog.hostWin.Close()

	assert.Equal(t, []bool{false}, responses)
	assert.Nil(t, d.dialog.hostWin)
	assert.Nil(t, w.Canvas().Overlays().Top(), "the scrim is removed on external close")
}

func TestDialog_WindowHosted_EscapeDismisses(t *testing.T) {
	if !windowHostSupported {
		t.Skip("window hosting is unsupported on this build target")
	}
	test.NewTempApp(t)
	w := test.NewTempWindow(t, nil)

	responses := []bool{}
	d := NewConfirm("Confirm", "Proceed?", func(ok bool) { responses = append(responses, ok) }, w)
	d.SetWindowHosted(true)
	d.Show()

	onKey := d.dialog.hostWin.Canvas().OnTypedKey()
	require.NotNil(t, onKey)
	onKey(&event.KeyEvent{Name: event.KeyEscape})

	assert.Equal(t, []bool{false}, responses, "Escape dismisses the dialog")
	assert.Nil(t, d.dialog.hostWin)
}

func TestDialog_WindowHosted_ResizeRoutesToWindow(t *testing.T) {
	if !windowHostSupported {
		t.Skip("window hosting is unsupported on this build target")
	}
	test.NewTempApp(t)
	w := test.NewTempWindow(t, nil)

	d := NewInformation("Title", "message", w)
	d.SetWindowHosted(true)
	d.Resize(geo.NewSize(300, 200))
	d.Show()
	defer d.Hide()

	dlg := d.(*dialog)
	assert.Equal(t, geo.NewSize(300, 200), dlg.hostWin.Canvas().Size())

	d.Resize(geo.NewSize(350, 250))
	assert.Equal(t, geo.NewSize(350, 250), dlg.hostWin.Canvas().Size())
}

func TestDialog_WindowHosted_ReShow(t *testing.T) {
	if !windowHostSupported {
		t.Skip("window hosting is unsupported on this build target")
	}
	test.NewTempApp(t)
	w := test.NewTempWindow(t, nil)

	d := NewInformation("Title", "message", w)
	d.SetWindowHosted(true)
	d.Show()
	dlg := d.(*dialog)
	first := dlg.hostWin
	d.Hide()

	d.Show()
	defer d.Hide()
	require.NotNil(t, dlg.hostWin, "a hidden dialog can be shown again")
	assert.NotSame(t, first, dlg.hostWin, "each show opens a fresh window")
	assert.Equal(t, dlg.scrim, w.Canvas().Overlays().Top())

	// A second Show while open focuses the existing window instead.
	second := dlg.hostWin
	d.Show()
	assert.Same(t, second, dlg.hostWin)
}

func TestDialog_WindowHosted_SetDismissTextWhileShown(t *testing.T) {
	if !windowHostSupported {
		t.Skip("window hosting is unsupported on this build target")
	}
	test.NewTempApp(t)
	w := test.NewTempWindow(t, nil)

	d := NewInformation("Title", "message", w)
	d.SetWindowHosted(true)
	d.Show()
	defer d.Hide()

	dlg := d.(*dialog)
	sizeBefore := dlg.body.Size()
	d.SetDismissText("Done")
	assert.Equal(t, sizeBefore, dlg.body.Size(),
		"refresh while hosted must not lay the body out to the hidden popup")
}

func TestColorPicker_WindowHosted_RebuildReseats(t *testing.T) {
	if !windowHostSupported {
		t.Skip("window hosting is unsupported on this build target")
	}
	test.NewTempApp(t)
	w := test.NewTempWindow(t, nil)

	p := NewColorPicker("Pick", "a color", nil, w)
	p.SetWindowHosted(true)
	p.Show()
	defer p.Hide()

	require.NotNil(t, p.dialog.hostWin)
	assert.Equal(t, p.dialog.body, p.dialog.hostWin.Content())

	// updateUI rebuilds the dialog body from scratch; the host window must
	// pick up the fresh body.
	p.Refresh()
	assert.Equal(t, p.dialog.body, p.dialog.hostWin.Content())
}

type fakeParentWindow struct {
	bellui.Window
	x, y int
}

func (f *fakeParentWindow) Position() (int, int)      { return f.x, f.y }
func (f *fakeParentWindow) SetOnMoved(func(int, int)) {}
func (f *fakeParentWindow) RequestPosition(int, int)  {}
func (f *fakeParentWindow) RequestAlwaysOnTop()       {}
func (f *fakeParentWindow) RequestFullScreenSecondary() {
}

type fakeChildWindow struct {
	bellui.Window
	x, y      int
	requested bool
}

func (f *fakeChildWindow) RequestPosition(x, y int) { f.x, f.y = x, y; f.requested = true }
func (f *fakeChildWindow) RequestAlwaysOnTop()      {}
func (f *fakeChildWindow) RequestFullScreenSecondary() {
}

type fakeModalWindow struct {
	bellui.Window
	parent bellui.Window
	modal  bool
}

func (f *fakeModalWindow) RequestPosition(int, int)    {}
func (f *fakeModalWindow) RequestAlwaysOnTop()         {}
func (f *fakeModalWindow) RequestFullScreenSecondary() {}
func (f *fakeModalWindow) SetParent(p bellui.Window)   { f.parent = p }
func (f *fakeModalWindow) SetModal(modal bool)         { f.modal = modal }

func TestAdoptModal(t *testing.T) {
	test.NewTempApp(t)
	parent := test.NewTempWindow(t, nil)

	// A driver window implementing desktop.ModalWindow gets both its parent
	// adopted and native modality enabled.
	child := &fakeModalWindow{}
	adoptModal(child, parent)
	assert.Same(t, parent, child.parent, "the parent is adopted")
	assert.True(t, child.modal, "native modality is enabled")

	// A window without the capability is left untouched (the scrim blocks the
	// parent instead) — adoptModal must not panic.
	plain := &fakeChildWindow{}
	adoptModal(plain, parent)
}

func TestCenterWindowOver(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewTempWindow(t, nil)
	w.Resize(geo.NewSize(600, 400)) // test canvas scale is 1.0: dp == native

	parent := &fakeParentWindow{Window: w, x: 100, y: 200}
	child := &fakeChildWindow{}

	centerWindowOver(child, parent, geo.NewSize(200, 100))
	assert.True(t, child.requested)
	assert.Equal(t, 100+(600-200)/2, child.x)
	assert.Equal(t, 200+(400-100)/2, child.y)

	// Plain windows without the desktop positioning interfaces are left to
	// the window manager.
	plain := &fakeChildWindow{}
	centerWindowOver(plain, w, geo.NewSize(200, 100))
	assert.True(t, plain.requested == false)
}
