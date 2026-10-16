package test

import (
	"os"
	"path/filepath"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/memcanvas"
)

// RenderObjectToMarkup renders the given [github.com/belluikit/bellui.CanvasObject] to a markup string.
//
// Since: 1.0
func RenderObjectToMarkup(o core.CanvasObject) string {
	c := NewCanvas()
	c.SetPadded(false)
	size := o.MinSize().Max(o.Size())
	c.SetContent(o)
	c.Resize(size) // ensure we are large enough for current size

	return snapshot(c)
}

// RenderToMarkup renders the given [github.com/belluikit/bellui.Canvas] to a markup string.
//
// Since: 1.0
func RenderToMarkup(c core.Canvas) string {
	return snapshot(c)
}

// Drag drags at an absolute position on the canvas.
// deltaX/Y is the dragging distance: <0 for dragging up/left, >0 for dragging down/right.
//
// The dispatched [github.com/belluikit/bellui/event.DragEvent] carries an
// AbsolutePosition of pos, matching the real driver, so widgets that resolve the
// cursor via AbsolutePosition − driver.AbsolutePositionForObject can be driven
// headlessly. Drag sends a single Dragged event followed immediately by DragEnd;
// use [DragSteps] to observe intermediate drag state.
func Drag(c core.Canvas, pos geo.Position, deltaX, deltaY float32) {
	matches := func(object core.CanvasObject) bool {
		_, ok := object.(event.Draggable)
		return ok
	}
	o, p, _ := intdriver.FindObjectAtPositionMatching(pos, matches, c.Overlays().List(), c.Content())
	if o == nil {
		return
	}
	e := &event.DragEvent{
		PointEvent: event.PointEvent{Position: p, AbsolutePosition: pos},
		Dragged:    geo.Delta{DX: deltaX, DY: deltaY},
	}
	o.(event.Draggable).Dragged(e)
	o.(event.Draggable).DragEnd()
}

// DragSteps performs a drag starting at absolute position pos and moving by the
// given delta, split into the requested number of Dragged events (minimum one),
// finishing with DragEnd. deltaX/Y is the total dragging distance: <0 for
// up/left, >0 for down/right.
//
// The draggable is resolved once at pos and receives every event of the gesture
// (mirroring the real driver, which keeps the drag's origin target for the whole
// gesture), and each event carries an AbsolutePosition that advances toward the
// target. If onStep is non-nil it is called after each Dragged event — while the
// drag is still in progress, before DragEnd — so tests can capture or assert
// mid-drag state such as ghost previews or drop highlights. onStep receives the
// 1-based step index.
//
// Since: 1.0
func DragSteps(c core.Canvas, pos geo.Position, deltaX, deltaY float32, steps int, onStep func(step int)) {
	if steps < 1 {
		steps = 1
	}
	matches := func(object core.CanvasObject) bool {
		_, ok := object.(event.Draggable)
		return ok
	}
	o, p, _ := intdriver.FindObjectAtPositionMatching(pos, matches, c.Overlays().List(), c.Content())
	if o == nil {
		return
	}
	drag := o.(event.Draggable)

	prev := pos
	for i := 1; i <= steps; i++ {
		frac := float32(i) / float32(steps)
		cur := pos.AddXY(deltaX*frac, deltaY*frac)
		e := &event.DragEvent{
			PointEvent: event.PointEvent{
				// The object does not move, so its local position tracks the
				// cursor's advance from the start.
				Position:         p.AddXY(cur.X-pos.X, cur.Y-pos.Y),
				AbsolutePosition: cur,
			},
			Dragged: geo.NewDelta(cur.X-prev.X, cur.Y-prev.Y),
		}
		drag.Dragged(e)
		if onStep != nil {
			onStep(i)
		}
		prev = cur
	}
	drag.DragEnd()
}

// FocusNext focuses the next focusable on the canvas.
func FocusNext(c core.Canvas) {
	if tc, ok := c.(*memcanvas.Canvas); ok {
		tc.FocusManager().FocusNext()
	} else {
		bellui.LogError("FocusNext can only be called with a test canvas", nil)
	}
}

// FocusPrevious focuses the previous focusable on the canvas.
func FocusPrevious(c core.Canvas) {
	if tc, ok := c.(*memcanvas.Canvas); ok {
		tc.FocusManager().FocusPrevious()
	} else {
		bellui.LogError("FocusPrevious can only be called with a test canvas", nil)
	}
}

// LaidOutObjects returns all core.CanvasObject starting at the given core.CanvasObject which is laid out previously.
func LaidOutObjects(o core.CanvasObject) (objects []core.CanvasObject) {
	if o != nil {
		objects = layoutAndCollect(objects, o, o.MinSize().Max(o.Size()))
	}
	return objects
}

// MoveMouse simulates a mouse movement to the given position.
func MoveMouse(c core.Canvas, pos geo.Position) {
	if bellui.CurrentDevice().IsMobile() {
		return
	}

	tc, _ := c.(*memcanvas.Canvas)
	var oldHovered, hovered event.Hoverable
	if tc != nil {
		oldHovered = tc.Hovered()
	}
	matches := func(object core.CanvasObject) bool {
		_, ok := object.(event.Hoverable)
		return ok
	}
	o, p, _ := intdriver.FindObjectAtPositionMatching(pos, matches, c.Overlays().List(), c.Content())
	if o != nil {
		hovered = o.(event.Hoverable)
		ev := &event.HoverEvent{
			PointEvent: event.PointEvent{
				AbsolutePosition: pos,
				Position:         p,
			},
		}
		if hovered == oldHovered {
			hovered.HoverMoved(ev)
		} else {
			if oldHovered != nil {
				oldHovered.HoverOut()
			}
			hovered.HoverIn(ev)
		}
	} else if oldHovered != nil {
		oldHovered.HoverOut()
	}
	if tc != nil {
		tc.SetHovered(hovered)
	}
}

// Scroll scrolls at an absolute position on the canvas.
// deltaX/Y is the scrolling distance: <0 for scrolling up/left, >0 for scrolling down/right.
func Scroll(c core.Canvas, pos geo.Position, deltaX, deltaY float32) {
	matches := func(object core.CanvasObject) bool {
		_, ok := object.(event.Scrollable)
		return ok
	}
	o, _, _ := intdriver.FindObjectAtPositionMatching(pos, matches, c.Overlays().List(), c.Content())
	if o == nil {
		return
	}

	e := &event.ScrollEvent{Scrolled: geo.Delta{DX: deltaX, DY: deltaY}}
	o.(event.Scrollable).Scrolled(e)
}

// DoubleTap simulates a double left mouse click on the specified object.
func DoubleTap(obj event.DoubleTappable) {
	ev, c := prepareTap(obj, geo.NewPos(1, 1))
	handleFocusOnTap(c, obj)
	obj.DoubleTapped(ev)
}

// Tap simulates a left mouse click on the specified object.
func Tap(obj event.Tappable) {
	TapAt(obj, geo.NewPos(1, 1))
}

// TapAt simulates a left mouse click on the passed object at a specified place within it.
func TapAt(obj event.Tappable, pos geo.Position) {
	ev, c := prepareTap(obj, pos)
	tap(c, obj, ev)
}

// TapCanvas taps at an absolute position on the canvas.
func TapCanvas(c core.Canvas, pos geo.Position) {
	if o, p := findTappable(c, pos); o != nil {
		tap(c, o.(event.Tappable), &event.PointEvent{AbsolutePosition: pos, Position: p})
	}
}

// TapSecondary simulates a right mouse click on the specified object.
func TapSecondary(obj event.SecondaryTappable) {
	TapSecondaryAt(obj, geo.NewPos(1, 1))
}

// TapSecondaryAt simulates a right mouse click on the passed object at a specified place within it.
func TapSecondaryAt(obj event.SecondaryTappable, pos geo.Position) {
	ev, c := prepareTap(obj, pos)
	handleFocusOnTap(c, obj)
	obj.TappedSecondary(ev)
}

// LongPress simulates a long-press gesture on the specified object.
func LongPress(obj event.LongPressable) {
	LongPressAt(obj, geo.NewPos(1, 1))
}

// LongPressAt simulates a long-press gesture on the passed object at a specified place within it.
func LongPressAt(obj event.LongPressable, pos geo.Position) {
	ev, c := prepareTap(obj, pos)
	handleFocusOnTap(c, obj)
	obj.LongPressed(ev)
}

// Type performs a series of key events to simulate typing of a value into the specified object.
// The focusable object will be focused before typing begins.
// The chars parameter will be input one rune at a time to the focused object.
func Type(obj event.Focusable, chars string) {
	obj.FocusGained()

	typeChars([]rune(chars), obj.TypedRune)
}

// TypeOnCanvas is like the Type function but it passes the key events to the canvas object
// rather than a focusable widget.
func TypeOnCanvas(c core.Canvas, chars string) {
	typeChars([]rune(chars), c.OnTypedRune())
}

// Compose sends an in-progress IME composition (preedit) update to the focused
// object of the canvas when it supports composition ([event.Composable]). It is
// a no-op if nothing focusable composable is focused.
//
// Since: 1.0
func Compose(c core.Canvas, text string, cursorPos int) {
	if comp, ok := c.Focused().(event.Composable); ok {
		comp.Compose(&event.PreeditEvent{Text: text, CursorPos: cursorPos})
	}
}

// ComposeCommit commits an IME composition on the focused composable object of
// the canvas, inserting text as if it had been typed. A no-op when the focused
// object does not support composition.
//
// Since: 1.0
func ComposeCommit(c core.Canvas, text string) {
	if comp, ok := c.Focused().(event.Composable); ok {
		comp.ComposeCommit(text)
	}
}

// ComposeCancel cancels any in-progress IME composition on the focused
// composable object of the canvas. A no-op when the focused object does not
// support composition.
//
// Since: 1.0
func ComposeCancel(c core.Canvas) {
	if comp, ok := c.Focused().(event.Composable); ok {
		comp.ComposeCancel()
	}
}

// WidgetRenderer allows test scripts to gain access to the current renderer for a widget.
// This can be used for verifying correctness of rendered components for a widget in unit tests.
func WidgetRenderer(wid core.Widget) core.WidgetRenderer {
	return cache.Renderer(wid)
}

func findTappable(c core.Canvas, pos geo.Position) (o core.CanvasObject, p geo.Position) {
	matches := func(object core.CanvasObject) bool {
		_, ok := object.(event.Tappable)
		return ok
	}
	o, p, _ = intdriver.FindObjectAtPositionMatching(pos, matches, c.Overlays().List(), c.Content())
	return o, p
}

func prepareTap(obj any, pos geo.Position) (*event.PointEvent, core.Canvas) {
	d := bellui.CurrentApp().Driver()
	ev := &event.PointEvent{Position: pos}
	var c core.Canvas
	if co, ok := obj.(core.CanvasObject); ok {
		c = d.CanvasForObject(co)
		ev.AbsolutePosition = d.AbsolutePositionForObject(co).Add(pos)
	}
	return ev, c
}

func tap(c core.Canvas, obj event.Tappable, ev *event.PointEvent) {
	handleFocusOnTap(c, obj)
	obj.Tapped(ev)
}

func handleFocusOnTap(c core.Canvas, obj any) {
	if c == nil {
		return
	}

	if focus, ok := obj.(event.Focusable); ok {
		dis, ok := obj.(core.Disableable)
		if (!ok || !dis.Disabled()) && focus == c.Focused() {
			return
		}
	}

	c.Unfocus()
}

func typeChars(chars []rune, keyDown func(rune)) {
	for _, char := range chars {
		keyDown(char)
	}
}

func writeMarkup(path string, markup string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(markup), 0o644)
}
