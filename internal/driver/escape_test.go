package driver_test

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// escapeOverlay is a minimal overlay that records whether it was asked to
// dismiss and answers as told.
type escapeOverlay struct {
	widget.BaseWidget
	dismisses bool
	asked     int
}

func newEscapeOverlay(dismisses bool) *escapeOverlay {
	o := &escapeOverlay{dismisses: dismisses}
	o.ExtendBaseWidget(o)
	return o
}

func (o *escapeOverlay) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (o *escapeOverlay) DismissOnEscape() bool {
	o.asked++
	return o.dismisses
}

func TestHandleEscape_NoOverlays(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewTempWindow(t, widget.NewLabel("content"))

	assert.False(t, driver.HandleEscape(w.Canvas()), "with nothing to close, Escape stays unconsumed")
}

func TestHandleEscape_ClosesTopOverlay(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewTempWindow(t, widget.NewLabel("content"))

	under, over := newEscapeOverlay(true), newEscapeOverlay(true)
	w.Canvas().Overlays().Add(under)
	w.Canvas().Overlays().Add(over)

	require.True(t, driver.HandleEscape(w.Canvas()))
	assert.Equal(t, 1, over.asked, "the innermost overlay is the one the user means to back out of")
	assert.Zero(t, under.asked, "the overlay beneath it is left alone")
}

func TestHandleEscape_SkipsOverlaysThatRefuse(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewTempWindow(t, widget.NewLabel("content"))

	dialog, snackbar := newEscapeOverlay(true), newEscapeOverlay(false)
	w.Canvas().Overlays().Add(dialog)
	w.Canvas().Overlays().Add(snackbar) // owns its own lifetime, declines

	require.True(t, driver.HandleEscape(w.Canvas()))
	assert.Equal(t, 1, snackbar.asked)
	assert.Equal(t, 1, dialog.asked, "a surface that declines passes the key down, it does not swallow it")
}

func TestHandleEscape_UnknownOverlayEndsTheWalk(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewTempWindow(t, widget.NewLabel("content"))

	dialog := newEscapeOverlay(true)
	w.Canvas().Overlays().Add(dialog)
	w.Canvas().Overlays().Add(widget.NewLabel("menu stand-in")) // not EscapeDismissible

	assert.False(t, driver.HandleEscape(w.Canvas()), "an overlay the walk cannot ask ends it, unconsumed")
	assert.Zero(t, dialog.asked, "nothing beneath it may be reached")
}

func TestHandleEscape_MenuOverDialogClosesTheMenu(t *testing.T) {
	// The regression case: a dropdown or context menu open above a modal
	// dialog. The menu's overlay is an OverlayContainer, not a PopUp — it does
	// not implement EscapeDismissible and closes itself through the focus
	// path. Escape must close the menu, never the dialog beneath it.
	test.NewTempApp(t)
	w := test.NewTempWindow(t, widget.NewLabel("content"))
	w.Resize(geo.NewSize(300, 200))

	dialogClosed := 0
	pop := widget.NewModalPopUp(widget.NewLabel("form"), w.Canvas())
	pop.OnDismissed = func() {
		dialogClosed++
		pop.Hide()
	}
	pop.Show()

	menu := widget.NewPopUpMenu(bellui.NewMenu("", bellui.NewMenuItem("A", func() {})), w.Canvas())
	menu.ShowAtPosition(geo.NewPos(10, 10))

	assert.False(t, driver.HandleEscape(w.Canvas()), "the walk stops at the menu's overlay")
	assert.Zero(t, dialogClosed, "the dialog underneath must not cancel")

	// The unconsumed key then takes the normal path. On desktop the menu holds
	// focus and closes itself — the behaviour menus have always had. On mobile
	// PopUpMenu.Show skips canvas.Focus, so there is no focus path; Escape
	// (a hardware key there) never closed menus on mobile and still does not.
	if bellui.CurrentDevice().IsMobile() {
		menu.Hide()
	} else {
		focused := w.Canvas().Focused()
		require.NotNil(t, focused, "the menu holds focus while open")
		focused.TypedKey(&event.KeyEvent{Name: event.KeyEscape})
		assert.False(t, menu.Visible(), "the menu closed")
		assert.True(t, pop.Visible(), "the dialog is still up")
	}

	// With the menu gone, Escape reaches the dialog as usual.
	require.True(t, driver.HandleEscape(w.Canvas()))
	assert.Equal(t, 1, dialogClosed)
}

func TestHandleEscape_FieldInsideThePopUpCanClaimIt(t *testing.T) {
	// The case that decides whether a dialog is escapable at all: a text field
	// inside it holds the focus, so it is the only thing the key can reach.
	test.NewTempApp(t)
	w := test.NewTempWindow(t, widget.NewLabel("content"))
	w.Resize(geo.NewSize(300, 200))

	entry := widget.NewEntry()
	pop := widget.NewModalPopUp(entry, w.Canvas())
	pop.Show()
	w.Canvas().Focus(entry)
	require.Same(t, entry, w.Canvas().Focused(), "the field inside the pop-up should hold focus")

	// A host that gave the field its own meaning for Escape keeps the key.
	entry.OnEscape = func() {}
	assert.False(t, driver.HandleEscape(w.Canvas()))
	assert.True(t, pop.Visible(), "the pop-up stays open while the field is using Escape")

	// A plain field does not trap the user: Escape closes the pop-up around it.
	entry.OnEscape = nil
	assert.True(t, driver.HandleEscape(w.Canvas()))
	assert.False(t, pop.Visible())
}

func TestHandleEscape_PopUpRunsItsHostCloseHook(t *testing.T) {
	// What a dialog relies on: dismissal goes through its own close path, which
	// is where the result is reported, rather than a bare Hide.
	test.NewTempApp(t)
	w := test.NewTempWindow(t, widget.NewLabel("content"))
	w.Resize(geo.NewSize(300, 200))

	closed := 0
	pop := widget.NewModalPopUp(widget.NewLabel("body"), w.Canvas())
	pop.OnDismissed = func() {
		closed++
		pop.Hide()
	}
	pop.Show()

	require.True(t, driver.HandleEscape(w.Canvas()))
	assert.Equal(t, 1, closed)
	assert.False(t, pop.Visible())
}
