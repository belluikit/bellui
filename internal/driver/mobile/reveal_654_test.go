//go:build !wasm

package mobile

import (
	"image/color"
	"testing"

	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
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
