package common

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// TestDroppedFocusedPageLosesFocus is #1089: a focused entry on a page the app
// swaps out of a container, with no bellui.Release, loses the focus by the
// frame that shows the next page, so no key typed after reaches it. The
// frame's walk visits only the changed paths, as one after a container change
// does.
func TestDroppedFocusedPageLosesFocus(t *testing.T) {
	test.NewTempApp(t)
	c, slot, entry := pagedCanvas(t, &screenState{})
	require.Equal(t, entry, c.Focused(), "precondition: the entry is focused")
	unfocused := 0
	c.OnUnfocus = func() { unfocused++ }
	next := widget.NewLabel("next")
	idleFrame(c) // takes in the layout change making next made

	full := c.lastFullMinSize
	setPage(c, slot, next)
	idleFrame(c)
	require.Equal(t, full, c.lastFullMinSize, "precondition: the frame walks only changed paths")

	assert.Nil(t, c.Focused(), "the dropped entry kept the focus")
	assert.Equal(t, 1, unfocused, "OnUnfocus was not called once")
}

// TestDroppedFocusedPageLosesFocusOnFullWalk: a full walk unfocuses it too.
func TestDroppedFocusedPageLosesFocusOnFullWalk(t *testing.T) {
	test.NewTempApp(t)
	c, slot, _ := pagedCanvas(t, &screenState{})

	slot.Objects = []core.CanvasObject{widget.NewLabel("next")} // with no refresh
	c.minSizeFull.Store(true)
	idleFrame(c)

	assert.Nil(t, c.Focused(), "the dropped entry kept the focus")
}

// TestMovedFocusedObjectKeepsFocus: an object moved to another container in
// the trees is still shown, and keeps the focus, although the walk dropped its
// old node.
func TestMovedFocusedObjectKeepsFocus(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewEntry()
	from := container.NewVBox(widget.NewLabel("a"), entry)
	to := container.NewVBox(widget.NewLabel("b"))
	c := walkedCanvas(container.NewHBox(from, to))
	c.Focus(entry)
	idleFrame(c)

	full := c.lastFullMinSize
	from.Remove(entry)
	to.Add(entry)
	c.Refresh(from)
	c.Refresh(to)
	idleFrame(c)
	require.Equal(t, full, c.lastFullMinSize, "precondition: the frame walks only changed paths")

	assert.Equal(t, entry, c.Focused(), "the moved entry lost the focus")
}

// TestObjectFocusedBeforeAWalkKeepsFocus: an object the app added and focused
// before a walk placed it is out of the node map too. A walk of changed paths
// that drops another object's node must not unfocus it. The object shows
// nothing new when focused, as an Entry's cursor does, which would make the
// frame a full walk.
func TestObjectFocusedBeforeAWalkKeepsFocus(t *testing.T) {
	test.NewTempApp(t)
	gone := widget.NewLabel("gone")
	box := container.NewVBox(gone)
	other := container.NewVBox()
	c := walkedCanvas(container.NewHBox(box, other))
	field := newPlainFocusable()
	idleFrame(c)

	full := c.lastFullMinSize
	other.Objects = append(other.Objects, field) // not refreshed: no walk visits it yet
	c.Focus(field)
	box.Remove(gone)
	c.Refresh(box)
	idleFrame(c)
	require.Equal(t, full, c.lastFullMinSize, "precondition: the frame walks only changed paths")

	assert.Equal(t, field, c.Focused(), "an object no walk had placed yet lost the focus")
}

// plainFocusable is a focusable widget that changes nothing when focused.
type plainFocusable struct{ widget.BaseWidget }

func newPlainFocusable() *plainFocusable {
	f := &plainFocusable{}
	f.ExtendBaseWidget(f)
	return f
}

func (f *plainFocusable) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Black))
}
func (f *plainFocusable) FocusGained()             {}
func (f *plainFocusable) FocusLost()               {}
func (f *plainFocusable) TypedRune(rune)           {}
func (f *plainFocusable) TypedKey(*event.KeyEvent) {}

// TestHiddenFocusedObjectKeepsFocus: hiding the focused object leaves it in
// the trees, and the focus where it was, as before #1089.
func TestHiddenFocusedObjectKeepsFocus(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewEntry()
	box := container.NewVBox(widget.NewLabel("a"), entry)
	c := walkedCanvas(box)
	c.Focus(entry)
	idleFrame(c)

	entry.Hide()
	c.Refresh(box)
	idleFrame(c)

	assert.Equal(t, entry, c.Focused(), "hiding the entry took its focus")
}
