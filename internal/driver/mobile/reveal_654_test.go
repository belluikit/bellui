//go:build !wasm

package mobile

import (
	"image/color"
	"testing"

	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/widget"
)

// A focused field is scrolled into the space above an on-screen keyboard,
// which a phone canvas reports as a bottom inset. The keyboard used to cover
// the field it was opened for, with nothing scrolling it into view (#654).
func TestCanvas_RevealsTheFocusedFieldAboveTheKeyboard(t *testing.T) {
	dev := &device{}
	c := newCanvas(dev).(*canvas)
	c.scale = 1

	spacer := func(h float32) core.CanvasObject {
		r := belluicanvas.NewRectangle(color.Transparent)
		r.SetMinSize(geo.NewSize(10, h))
		return r
	}
	above := widget.NewEntry()
	field := widget.NewEntry()
	c.SetContent(container.NewVScroll(container.NewVBox(above, spacer(900), field, spacer(400))))
	c.Resize(geo.NewSize(400, 600))

	fieldSpan := func() (top, bottom float32) {
		p := intdriver.AbsolutePositionForObject(field, []core.CanvasObject{c.Content()})
		return p.Y, p.Y + field.Size().Height
	}
	visibleBottom := func() float32 {
		pos, size := c.InteractiveArea()
		return pos.Y + size.Height
	}

	c.Focus(above) // already in view: nothing scrolls
	if top, _ := fieldSpan(); top < 900 {
		t.Fatalf("focusing a field in view scrolled the page: the lower field is at %v", top)
	}

	c.Focus(field)
	top, bottom := fieldSpan()
	if bottom > visibleBottom() || top < 0 {
		t.Fatalf("the focused field spans %v-%v, outside the canvas's 0-%v", top, bottom, visibleBottom())
	}

	dev.safeBottom = 300 // the keyboard opens
	c.sizeContent(c.size)
	c.revealFocused() // what the driver does on an inset change
	top, bottom = fieldSpan()
	if bottom > visibleBottom() {
		t.Errorf("the focused field's bottom is at %v, under the keyboard, which starts at %v", bottom, visibleBottom())
	}
	if top < 0 {
		t.Errorf("the field scrolled past the top, to %v", top)
	}
}

// Scrolling with the keyboard up keeps it up, so what it covers can be
// scrolled into view; a tap away from the field still closes it. The focus
// used to go at touch-down, so starting to scroll closed the keyboard (#654).
func TestCanvas_ScrollKeepsTheFocusATapTakesIt(t *testing.T) {
	c := newCanvas(&device{}).(*canvas)
	c.scale = 1
	field := widget.NewEntry()
	filler := belluicanvas.NewRectangle(color.Transparent)
	filler.SetMinSize(geo.NewSize(10, 2000))
	c.SetContent(container.NewVScroll(container.NewVBox(field, filler)))
	c.Resize(geo.NewSize(400, 600))
	c.Focus(field)

	drag := func(d event.Draggable, ev *event.DragEvent) { d.Dragged(ev) }
	from, to := geo.NewPos(200, 400), geo.NewPos(200, 300)
	c.TapDown(from, 1)
	c.TapMove(to, 1, drag)
	c.TapUp(to, 1, nil, nil, nil, nil, func(d event.Draggable, _ *event.DragEvent) { d.DragEnd() })
	if c.Focused() != field {
		t.Fatalf("a scroll took the focus, and the keyboard, from the field: focused %v", c.Focused())
	}

	c.TapDown(from, 2)
	c.TapUp(from, 2, func(event.Tappable, *event.PointEvent) {}, nil, nil, nil, nil)
	if c.Focused() != nil {
		t.Errorf("a tap away from the field left it focused")
	}
}
