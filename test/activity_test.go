package test_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// Each helper that simulates the user's input moves LastUserActivity forward,
// as the real drivers do when that input arrives (#1111).
func TestInputHelpers_StampUserActivity(t *testing.T) {
	for name, input := range map[string]func(c test.WindowlessCanvas, o *inputTarget){
		"Drag":           func(c test.WindowlessCanvas, _ *inputTarget) { test.Drag(c, geo.NewPos(5, 5), 10, 0) },
		"DragSteps":      func(c test.WindowlessCanvas, _ *inputTarget) { test.DragSteps(c, geo.NewPos(5, 5), 10, 0, 3, nil) },
		"Scroll":         func(c test.WindowlessCanvas, _ *inputTarget) { test.Scroll(c, geo.NewPos(5, 5), 0, -40) },
		"Tap":            func(_ test.WindowlessCanvas, o *inputTarget) { test.Tap(o) },
		"TapAt":          func(_ test.WindowlessCanvas, o *inputTarget) { test.TapAt(o, geo.NewPos(5, 5)) },
		"TapCanvas":      func(c test.WindowlessCanvas, _ *inputTarget) { test.TapCanvas(c, geo.NewPos(5, 5)) },
		"DoubleTap":      func(_ test.WindowlessCanvas, o *inputTarget) { test.DoubleTap(o) },
		"TapSecondary":   func(_ test.WindowlessCanvas, o *inputTarget) { test.TapSecondary(o) },
		"TapSecondaryAt": func(_ test.WindowlessCanvas, o *inputTarget) { test.TapSecondaryAt(o, geo.NewPos(5, 5)) },
		"LongPress":      func(_ test.WindowlessCanvas, o *inputTarget) { test.LongPress(o) },
		"LongPressAt":    func(_ test.WindowlessCanvas, o *inputTarget) { test.LongPressAt(o, geo.NewPos(5, 5)) },
		"Type":           func(_ test.WindowlessCanvas, o *inputTarget) { test.Type(o, "a") },
		"TypeOnCanvas":   func(c test.WindowlessCanvas, _ *inputTarget) { test.TypeOnCanvas(c, "a") },
		"Compose":        func(c test.WindowlessCanvas, _ *inputTarget) { test.Compose(c, "ka", 2) },
		"ComposeCommit":  func(c test.WindowlessCanvas, _ *inputTarget) { test.ComposeCommit(c, "か") },
		"ComposeCancel":  func(c test.WindowlessCanvas, _ *inputTarget) { test.ComposeCancel(c) },
	} {
		t.Run(name, func(t *testing.T) {
			c, o := newInputCanvas(t)
			assert.True(t, bellui.LastUserActivity().IsZero(), "a new app starts with no activity")

			input(c, o)
			assert.False(t, bellui.LastUserActivity().IsZero(), "%s counts as user input", name)
			assert.Positive(t, o.events, "%s reached the target, so it ran as a user's input would", name)
		})
	}
}

// Input that lands on nothing still counts: the drivers stamp before they
// look for a target.
func TestInputHelpers_StampUserActivityWithNoTarget(t *testing.T) {
	test.NewTempApp(t)
	c := test.NewCanvas()
	c.SetContent(canvas.NewRectangle(nil))
	c.Resize(geo.NewSize(50, 50))

	test.Scroll(c, geo.NewPos(5, 5), 0, -40)
	assert.False(t, bellui.LastUserActivity().IsZero())
}

// What the drivers do not count, the helpers do not either: a mouse moved
// with no button held, and a widget method called directly from code.
func TestInputHelpers_NotUserActivity(t *testing.T) {
	c, o := newInputCanvas(t)

	test.MoveMouse(c, geo.NewPos(5, 5))
	o.Tapped(&event.PointEvent{})
	o.Scrolled(&event.ScrollEvent{})
	assert.True(t, bellui.LastUserActivity().IsZero())
}

// A new app starts at the zero time, so one test's input does not leak into
// the next.
func TestNewApp_ResetsUserActivity(t *testing.T) {
	_, o := newInputCanvas(t)
	test.Tap(o)
	assert.False(t, bellui.LastUserActivity().IsZero())

	test.NewApp()
	assert.True(t, bellui.LastUserActivity().IsZero())
}

func newInputCanvas(t *testing.T) (test.WindowlessCanvas, *inputTarget) {
	t.Helper()
	test.NewTempApp(t)
	o := &inputTarget{Rectangle: canvas.NewRectangle(nil)}
	w := test.NewTempWindow(t, o)
	w.Resize(geo.NewSize(50, 50))
	o.Resize(geo.NewSize(50, 50))
	c := w.Canvas().(test.WindowlessCanvas)
	c.Focus(o)
	c.SetOnTypedRune(func(rune) { o.events++ })
	return c, o
}

// inputTarget receives every kind of input the helpers simulate and counts
// what reached it.
type inputTarget struct {
	*canvas.Rectangle
	events int
}

func (i *inputTarget) Tapped(*event.PointEvent)          { i.events++ }
func (i *inputTarget) TappedSecondary(*event.PointEvent) { i.events++ }
func (i *inputTarget) DoubleTapped(*event.PointEvent)    { i.events++ }
func (i *inputTarget) LongPressed(*event.PointEvent)     { i.events++ }
func (i *inputTarget) Dragged(*event.DragEvent)          { i.events++ }
func (i *inputTarget) DragEnd()                          {}
func (i *inputTarget) Scrolled(*event.ScrollEvent)       { i.events++ }
func (i *inputTarget) FocusGained()                      {}
func (i *inputTarget) FocusLost()                        {}
func (i *inputTarget) TypedRune(rune)                    { i.events++ }
func (i *inputTarget) TypedKey(*event.KeyEvent)          { i.events++ }
func (i *inputTarget) Compose(*event.PreeditEvent)       { i.events++ }
func (i *inputTarget) ComposeCommit(string)              { i.events++ }
func (i *inputTarget) ComposeCancel()                    { i.events++ }
