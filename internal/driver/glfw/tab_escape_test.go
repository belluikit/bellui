//go:build !no_glfw && !mobile

// No !ci here, unlike window_test.go: this runs in the browser leg of CI too,
// which is the point. On the web Ctrl+Tab never reaches the page, so Escape
// then Tab is the only way out of a widget that keeps Tab there (#357).

package glfw

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

type escFocusable struct {
	canvas.Rectangle
	name string
}

func (*escFocusable) FocusGained()             {}
func (*escFocusable) FocusLost()               {}
func (*escFocusable) TypedRune(rune)           {}
func (*escFocusable) TypedKey(*event.KeyEvent) {}

type escKeepsTab struct {
	escFocusable
	tabs int
}

func (*escKeepsTab) AcceptsTab() bool { return true }

func (k *escKeepsTab) TypedKey(ev *event.KeyEvent) {
	if ev.Name == event.KeyTab {
		k.tabs++
	}
}

// keyWindow is a window with only what processKeyPressed needs: a canvas.
func keyWindow(objs ...core.CanvasObject) *window {
	c := newCanvas()
	c.SetContent(container.NewVBox(objs...))
	c.Resize(geo.NewSize(200, 200))
	return &window{canvas: c}
}

func (w *window) press(key event.KeyName, mods event.KeyModifier) {
	w.processKeyPressed(key, key, 0, press, mods)
}

func TestWindow_EscapeThenTabLeavesWidgetThatKeepsTab(t *testing.T) {
	before, after := &escFocusable{name: "before"}, &escFocusable{name: "after"}
	field := &escKeepsTab{}
	w := keyWindow(before, field, after)
	c := w.canvas

	c.Focus(field)
	w.press(event.KeyTab, 0)
	assert.Equal(t, field, c.Focused(), "plain Tab stays with the field")
	assert.Equal(t, 1, field.tabs)

	w.press(event.KeyEscape, 0)
	w.press(event.KeyTab, 0)
	assert.Equal(t, after, c.Focused(), "Escape then Tab moves to the next widget")
	assert.Equal(t, 1, field.tabs, "the Tab that leaves is not typed into the field")

	c.Focus(field)
	w.press(event.KeyEscape, 0)
	w.press(desktop.KeyShiftLeft, event.KeyModifierShift) // Shift goes down first
	w.press(event.KeyTab, event.KeyModifierShift)
	assert.Equal(t, before, c.Focused(), "Escape then Shift+Tab moves to the previous widget")

	c.Focus(field)
	w.press(event.KeyEscape, 0)
	w.press(event.KeyA, 0)
	w.press(event.KeyTab, 0)
	assert.Equal(t, field, c.Focused(), "a key between Escape and Tab cancels the way out")

	w.press(event.KeyTab, 0)
	assert.Equal(t, field, c.Focused(), "the way out is used once")
}

// The first Escape keeps its old meaning and closes the dialog; only an Escape
// with nothing left to close arms the way out. The case that matters is a
// field that keeps Tab inside the dialog: the Escape that closes the dialog
// must not also free the next Tab in the field underneath it.
func TestWindow_EscapeClosesAPopUpBeforeItFreesTab(t *testing.T) {
	field := &escKeepsTab{}
	after := &escFocusable{name: "after"}
	w := keyWindow(field, after)
	c := w.canvas
	c.Focus(field)

	inDialog := &escKeepsTab{}
	pop := widget.NewPopUp(inDialog, c)
	pop.Show()
	c.Focus(inDialog)
	assert.Equal(t, inDialog, c.Focused())

	w.press(event.KeyEscape, 0)
	assert.False(t, pop.Visible(), "the first Escape closes the dialog")
	assert.Equal(t, field, c.Focused(), "focus is back on the field underneath")
	w.press(event.KeyTab, 0)
	assert.Equal(t, field, c.Focused(), "that Escape did not also free Tab")
	assert.Equal(t, 1, field.tabs)

	w.press(event.KeyEscape, 0)
	w.press(event.KeyTab, 0)
	assert.Equal(t, after, c.Focused(), "a second Escape does")
}

// On the web letter keys reach processKeyPressed with no name, the same ""
// an unnamed modifier has; the character that follows is what cancels the way
// out. And a held Escape's repeats do not undo the press.
func TestWindow_EscapeThenTabEdges(t *testing.T) {
	field := &escKeepsTab{}
	after := &escFocusable{name: "after"}
	w := keyWindow(field, after)
	c := w.canvas

	c.Focus(field)
	w.press(event.KeyEscape, 0)
	w.press(event.KeyUnknown, 0) // a letter on the web
	w.processCharInput('a')
	w.press(event.KeyTab, 0)
	assert.Equal(t, field, c.Focused(), "typing after Escape cancels the way out")

	w.press(event.KeyEscape, 0)
	w.processKeyPressed(event.KeyEscape, event.KeyEscape, 0, repeat, 0)
	w.processKeyPressed(event.KeyEscape, event.KeyEscape, 0, repeat, 0)
	w.press(event.KeyTab, 0)
	assert.Equal(t, after, c.Focused(), "a held Escape still frees Tab")

	other := &escKeepsTab{}
	w = keyWindow(field, other, after)
	c = w.canvas
	c.Focus(field)
	w.press(event.KeyEscape, 0)
	c.Focus(other) // a click, not a key
	w.press(event.KeyTab, 0)
	assert.Equal(t, other, c.Focused(), "an Escape in one field does not free Tab in another")
	assert.Equal(t, 1, other.tabs)
}

// Leaving the window, or the browser tab, between Escape and Tab cancels it.
func TestWindow_EscapeThenFocusLossKeepsTab(t *testing.T) {
	field := &escKeepsTab{}
	after := &escFocusable{name: "after"}
	w := keyWindow(field, after)
	c := w.canvas

	c.Focus(field)
	w.press(event.KeyEscape, 0)
	w.processFocused(false)
	c.Focus(field)
	w.press(event.KeyTab, 0)
	assert.Equal(t, field, c.Focused())
}
