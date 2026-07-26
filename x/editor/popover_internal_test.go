package editor

import (
	"testing"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// renderedPopoverEditor returns an editor laid out in a window so that row and
// column geometry is populated, ready to anchor a popover.
func renderedPopoverEditor(t *testing.T, text string) *Editor {
	t.Helper()
	e := NewEditor()
	e.SetText(text)
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 300))
	return e
}

func newTestPopover() *popover {
	return newPopover(widget.NewLabel("alpha\nbeta\ngamma"))
}

func TestPopoverPosition(t *testing.T) {
	const vw, vh = float32(200), float32(100)
	bounds := geo.NewRectangle(0, 0, vw, vh)
	cell := func(x, y float32) geo.Rectangle { return geo.NewRectangle(x, y, 8, 12) }

	tests := []struct {
		name      string
		anchor    geo.Rectangle
		size      geo.Size
		wantX     float32
		wantY     float32
		wantPlace popoverPlacement
	}{
		{
			name:      "sits below when there is room",
			anchor:    cell(20, 10),
			size:      geo.NewSize(80, 40),
			wantX:     20,
			wantY:     22, // 10 + 12
			wantPlace: placeBelow,
		},
		{
			name:      "flips above near the bottom edge",
			anchor:    cell(20, 80),
			size:      geo.NewSize(80, 40),
			wantX:     20,
			wantY:     40, // 80 - 40
			wantPlace: placeAbove,
		},
		{
			name:      "stays below when below has more room even if it overflows",
			anchor:    cell(20, 40),
			size:      geo.NewSize(80, 60),
			wantX:     20,
			wantY:     52, // 40 + 12
			wantPlace: placeBelow,
		},
		{
			name:      "clamps to the right edge",
			anchor:    cell(180, 10),
			size:      geo.NewSize(80, 40),
			wantX:     120, // 200 - 80
			wantY:     22,
			wantPlace: placeBelow,
		},
		{
			name:      "clamps to the left edge when wider than viewport",
			anchor:    cell(10, 10),
			size:      geo.NewSize(250, 40),
			wantX:     0, // 200 - 250 = -50, clamped to 0
			wantY:     22,
			wantPlace: placeBelow,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pos, place := popoverPosition(tc.anchor, tc.size, bounds, false)
			if pos.X != tc.wantX || pos.Y != tc.wantY {
				t.Errorf("position = (%v,%v), want (%v,%v)", pos.X, pos.Y, tc.wantX, tc.wantY)
			}
			if place != tc.wantPlace {
				t.Errorf("placement = %v, want %v", place, tc.wantPlace)
			}
		})
	}
}

// TestPopoverPosition_OverflowsEditorWithinCanvas covers #19: when bounds (the
// host canvas area) are larger than the editor, a popover may extend past the
// editor's own height/width instead of being clamped to it.
func TestPopoverPosition_OverflowsEditorWithinCanvas(t *testing.T) {
	cell := func(x, y float32) geo.Rectangle { return geo.NewRectangle(x, y, 8, 12) }
	// Editor is ~100 tall, but the surrounding canvas is 500 tall.
	bounds := geo.NewRectangle(0, 0, 200, 500)

	// A tall list anchored near the editor's bottom stays below, overflowing the
	// editor into the canvas (would have flipped/clamped under the old editor
	// clamp).
	pos, place := popoverPosition(cell(20, 90), geo.NewSize(80, 200), bounds, false)
	if pos.Y != 102 || place != placeBelow { // 90 + 12
		t.Errorf("tall popover near editor bottom: got (y=%v, %v), want (102, below)", pos.Y, place)
	}

	// It still clamps to the canvas extent, not the editor.
	pos, place = popoverPosition(cell(180, 470), geo.NewSize(80, 40), bounds, false)
	if pos.X != 120 { // 200 - 80
		t.Errorf("x clamp to canvas right: got %v, want 120", pos.X)
	}
	if place != placeAbove || pos.Y != 430 { // 470+12=482, roomBelow 18 < 40 -> flip, 470-40
		t.Errorf("flip near canvas bottom: got (y=%v, %v), want (430, above)", pos.Y, place)
	}
}

// TestPopoverPosition_NegativeOriginBounds covers the embedded-editor case where
// the canvas origin sits above/left of the editor, so bounds has a negative min
// corner and the popover may be placed at a negative editor-local coordinate.
func TestPopoverPosition_NegativeOriginBounds(t *testing.T) {
	cell := func(x, y float32) geo.Rectangle { return geo.NewRectangle(x, y, 8, 12) }
	bounds := geo.NewRectangle(-40, -30, 200, 100) // canvas starts 40px left, 30px above

	// Anchor near the editor top: room above reaches up to y=-30, so a flip can
	// land at a negative editor-local Y.
	pos, place := popoverPosition(cell(10, 5), geo.NewSize(60, 50), bounds, false)
	if place != placeBelow || pos.Y != 17 { // fits below (room 100-30-17=53 >= 50)
		t.Errorf("got (y=%v, %v), want (17, below)", pos.Y, place)
	}

	// Force a flip by anchoring low; the flipped top may go negative but not past
	// the bounds min.
	pos, place = popoverPosition(cell(10, 55), geo.NewSize(60, 90), bounds, false)
	if place != placeAbove {
		t.Errorf("placement = %v, want above", place)
	}
	if pos.Y < -30 {
		t.Errorf("y=%v escaped the bounds min (-30)", pos.Y)
	}
}

// TestPopoverSizeWithin_CapsToMaxAndBounds verifies the popover size is clamped
// to its configured max and to the bounds extent.
func TestPopoverSizeWithin_CapsToMaxAndBounds(t *testing.T) {
	test.NewTempApp(t)
	p := newTestPopover()

	// Generous bounds: natural size passes through (sanity: positive).
	big := geo.NewRectangle(0, 0, 10000, 10000)
	natural := p.sizeWithin(big)
	if natural.Width <= 0 || natural.Height <= 0 {
		t.Fatalf("natural size should be positive, got %v", natural)
	}

	// maxHeight caps height.
	p.maxHeight = 10
	if got := p.sizeWithin(big); got.Height != 10 {
		t.Errorf("maxHeight cap: height = %v, want 10", got.Height)
	}

	// Tiny bounds cap both dimensions below the max.
	p.maxHeight = 0
	tiny := geo.NewRectangle(0, 0, 5, 7)
	if got := p.sizeWithin(tiny); got.Width != 5 || got.Height != 7 {
		t.Errorf("bounds cap: size = %v, want (5,7)", got)
	}
}

func TestPopover_ShowAndDismiss(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "abcdef\nghijkl\nmnopqr")
	p := newTestPopover()

	if !e.showPopoverAt(p, 0, 0) {
		t.Fatal("showPopoverAt returned false for a visible cell")
	}
	if !e.popoverVisible() {
		t.Fatal("popoverVisible should be true after show")
	}
	if len(e.floatLayer.Objects) != 1 {
		t.Fatalf("float layer should hold 1 object, got %d", len(e.floatLayer.Objects))
	}

	dismissed := false
	p.onDismiss = func() { dismissed = true }
	e.dismissPopover()
	if e.popoverVisible() {
		t.Error("popoverVisible should be false after dismiss")
	}
	if len(e.floatLayer.Objects) != 0 {
		t.Errorf("float layer should be empty after dismiss, got %d", len(e.floatLayer.Objects))
	}
	if !dismissed {
		t.Error("onDismiss should have fired")
	}
}

func TestPopover_ShowAtInvisibleCellFails(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "one line")

	if e.showPopoverAt(newTestPopover(), 99, 0) {
		t.Error("showPopoverAt should return false for an out-of-range cell")
	}
	if e.popoverVisible() {
		t.Error("no popover should be visible after a failed show")
	}
}

func TestPopover_EscapeDismisses(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "abcdef")
	e.showPopoverAt(newTestPopover(), 0, 0)

	if !e.entry.onKey(&event.KeyEvent{Name: event.KeyEscape}) {
		t.Error("Escape should be consumed while a popover is open")
	}
	if e.popoverVisible() {
		t.Error("Escape should dismiss the popover")
	}
}

func TestPopover_KeyHandlerIntercepts(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "abcdef")
	p := newTestPopover()
	seen := 0
	p.keyHandler = func(ev *event.KeyEvent) bool {
		if ev.Name == event.KeyDown {
			seen++
			return true
		}
		return false
	}
	e.showPopoverAt(p, 0, 0)

	if !e.entry.onKey(&event.KeyEvent{Name: event.KeyDown}) {
		t.Error("Down should be consumed by the popover key handler")
	}
	if seen != 1 {
		t.Errorf("key handler should have seen 1 event, got %d", seen)
	}
	if !e.popoverVisible() {
		t.Error("an intercepted key should not dismiss the popover")
	}
	// A key the handler ignores still falls through to Escape handling.
	if !e.entry.onKey(&event.KeyEvent{Name: event.KeyEscape}) || e.popoverVisible() {
		t.Error("Escape should still dismiss when the handler ignores it")
	}
}

func TestPopover_OutsideClickDismisses(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "abcdef\nghijkl")
	p := newTestPopover()
	e.showPopoverAt(p, 0, 0)

	// A click within the popover bounds must not dismiss it.
	inside := geo.NewPos(
		p.obj.Position().X+p.obj.Size().Width/2,
		p.obj.Position().Y+p.obj.Size().Height/2,
	)
	e.entry.onMouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: inside}, Button: desktop.MouseButtonPrimary})
	if !e.popoverVisible() {
		t.Fatal("a click inside the popover should not dismiss it")
	}

	// A click well outside dismisses.
	outside := geo.NewPos(p.obj.Position().X+p.obj.Size().Width+50, p.obj.Position().Y+p.obj.Size().Height+50)
	e.entry.onMouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: outside}, Button: desktop.MouseButtonPrimary})
	if e.popoverVisible() {
		t.Error("a click outside the popover should dismiss it")
	}
}

func TestPopover_FocusLossDismisses(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "abcdef")
	e.showPopoverAt(newTestPopover(), 0, 0)

	e.entry.onFocusLost()
	if e.popoverVisible() {
		t.Error("losing focus should dismiss the popover")
	}
}

func TestPopover_ScrollOffDismisses(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "a\nb\nc\nd\ne")
	e.showPopoverAt(newTestPopover(), 0, 0)

	// Scroll the anchor row far out of the viewport.
	e.scrollY = 10000
	e.repositionPopover()
	if e.popoverVisible() {
		t.Error("scrolling the anchor off-screen should dismiss the popover")
	}
}

func TestPopover_TracksAnchorOnScroll(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "a\nb\nc\nd\ne\nf\ng\nh")
	p := newTestPopover()
	e.showPopoverAt(p, 3, 0)
	before := p.obj.Position().Y

	// A small downward scroll moves the anchor — and the popover — up.
	e.scrollY = 6
	e.repositionPopover()
	if !e.popoverVisible() {
		t.Fatal("popover should still be visible after a small scroll")
	}
	if after := p.obj.Position().Y; after >= before {
		t.Errorf("popover Y should decrease after scrolling down: before=%v after=%v", before, after)
	}
}

func TestPopover_NilSafeWithoutPopover(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "abcdef")

	// With no popover open, the key hook declines (so Escape reaches the find
	// bar) and the mouse/focus hooks are no-ops.
	if e.entry.onKey(&event.KeyEvent{Name: event.KeyEscape}) {
		t.Error("onKey should decline when no popover is open")
	}
	e.entry.onMouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: geo.NewPos(10, 10)}, Button: desktop.MouseButtonPrimary}) // must not panic
	e.entry.onFocusLost()                                                                                                                    // must not panic
	e.repositionPopover()                                                                                                                    // must not panic
	if e.popoverVisible() {
		t.Error("no popover should be visible")
	}
}

func TestIntersectRect(t *testing.T) {
	a := geo.NewRectangle(0, 0, 100, 100)
	b := geo.NewRectangle(20, 30, 200, 40)
	got := intersectRect(a, b)
	if got != geo.NewRectangle(20, 30, 80, 40) {
		t.Errorf("intersect = %+v, want (20,30,80,40)", got)
	}
	// Negative-origin overlap (canvas above/left of the editor).
	got = intersectRect(geo.NewRectangle(-40, -30, 200, 100), geo.NewRectangle(-10, -10, 50, 50))
	if got != geo.NewRectangle(-10, -10, 50, 50) {
		t.Errorf("nested intersect = %+v, want (-10,-10,50,50)", got)
	}
}

// TestPopoverBounds_NoClipIsFullCanvas covers #19: with no clipping ancestor,
// popoverBounds is the whole canvas (so popovers can overflow the editor).
func TestPopoverBounds_NoClipIsFullCanvas(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText("abc")
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))

	b := e.popoverBounds()
	cs := w.Canvas().Size()
	if b.Size.Width != cs.Width || b.Size.Height != cs.Height {
		t.Errorf("unclipped bounds = %+v, want canvas size %v", b, cs)
	}
}

// TestPopoverBounds_ClampedToHostScroll covers #73: when a host wraps the editor
// in a container.Scroll smaller than the window, popoverBounds clamps to that
// scroll's viewport rather than the full canvas.
func TestPopoverBounds_ClampedToHostScroll(t *testing.T) {
	test.NewTempApp(t)
	captureLog(t) // this test intentionally nests the editor in a scroll (#111 guard warns)
	e := NewEditor()
	e.SetText("abc\ndef\nghi")
	scroll := container.NewScroll(e)
	// Force the scroll smaller than the window so its clip is a strict subset.
	host := container.NewGridWrap(geo.NewSize(180, 90), scroll)
	w := test.NewWindow(container.NewPadded(host))
	defer w.Close()
	w.Resize(geo.NewSize(600, 500))

	b := e.popoverBounds()
	if b.Size.Width > 180 || b.Size.Height > 90 {
		t.Errorf("bounds %+v should be clamped to the host scroll viewport (<=180x90)", b)
	}
	cs := w.Canvas().Size()
	if b.Size.Width >= cs.Width || b.Size.Height >= cs.Height {
		t.Errorf("bounds %+v should be a strict subset of the canvas %v", b, cs)
	}
}
