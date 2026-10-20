//go:build mobile && (!windows || !ci) && !wasm

package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// Selection handles through the real touch pipeline (pre-rebrand tracker #758): after a hold-fire
// long-press the handle overlay is on the canvas; a touch on a handle keeps
// the entry focused (the RetainsFocus seam) and dragging it adjusts the
// selection. The window comes from the shared test driver so the widget's
// CanvasForObject lookup resolves.
//
// Assertions inside waitAndCheck run on the driver loop goroutine — assert
// only, never require (FailNow would Goexit the loop and deadlock the test).
func Test_canvas_SelectionHandleDragAdjustsSelection(t *testing.T) {
	w := d.CreateWindow("selhandles").(*window)
	t.Cleanup(w.Close)
	e := widget.NewMultiLineEntry()
	w.SetContent(e)
	c := w.canvas
	c.Resize(geo.NewSize(300, 150))
	e.SetText("hello world this is a test\nsecond line of text here")

	// Long-press on "hello" via the hold-fire path.
	entryAbs := bellui.CurrentApp().Driver().AbsolutePositionForObject(e)
	pos := entryAbs.AddXY(20, 10)
	c.TapDown(pos, 0)
	waitAndCheck(400, func() {
		assert.Equal(t, "hello", e.SelectedText(), "hold-fire selects the word")
	})
	lprTapUp(c, pos)

	waitAndCheck(100, func() {
		assert.GreaterOrEqual(t, len(c.Overlays().List()), 2, "handle overlay + context menu on the canvas")
	})
	if t.Failed() {
		return
	}

	// Dismiss the context menu (a user taps outside it; how the popup handles
	// that is its own widget's concern). The handles overlay is
	// input-pass-through, so with the menu gone the entry owns the keyboard
	// again (TopFocusManager skips pass-through overlays, pre-rebrand tracker #569).
	waitAndCheck(50, func() {
		c.Overlays().Remove(c.Overlays().Top())
		assert.Len(t, c.Overlays().List(), 1, "only the handle overlay remains")
		assert.Equal(t, e, c.Focused(), "the entry owns the keyboard")
	})
	if t.Failed() {
		return
	}

	// Grab the end handle: it hangs just below the selection end. The driver
	// resolves it through the pass-through overlay.
	endRect := e.CaretRect(0, 5)
	grab := entryAbs.Add(endRect.Pos).AddXY(0, endRect.Size.Height+10)
	c.TapDown(grab, 1)
	waitAndCheck(50, func() {
		assert.Equal(t, e, c.Focused(), "touching a handle keeps the entry focused")
	})

	// Drag it to the right, past " world". The drag target locks on the
	// first past-slop move, so step off the handle gradually the way a real
	// finger does.
	target := e.CaretRect(0, 11)
	// Aim a hair left of the column-11 caret so the half-char hit-test
	// rounding cannot land on column 12.
	dest := entryAbs.Add(target.Pos).AddXY(-2, target.Size.Height+10)
	moved := false
	drag := func(wid event.Draggable, ev *event.DragEvent) {
		moved = true
		wid.Dragged(ev)
	}
	c.TapMove(grab.AddXY(6, 0), 1, drag)
	c.TapMove(dest, 1, drag)
	waitAndCheck(50, func() {
		assert.True(t, moved, "the handle claims the drag")
		assert.Equal(t, "hello world", e.SelectedText(), "dragging the end handle extends the selection")
	})
}
