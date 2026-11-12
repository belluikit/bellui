package driver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
)

// bandOverlay covers the canvas but leaves the band above top to the content
// beneath, as a drawer below a header does (#1171).
type bandOverlay struct {
	objectTree
	top float32
}

func (o *bandOverlay) PassesInputThroughAt(pos geo.Position) bool { return pos.Y < o.top }

// Above the band's top an overlay is skipped as if absent; below it the
// overlay is the barrier a modal one is, even where it matches nothing.
func TestFindObjectAtPosition_PassesInputThroughAt(t *testing.T) {
	button := &objectTree{pos: geo.NewPos(0, 0), size: geo.NewSize(100, 40)}
	content := &objectTree{children: []core.CanvasObject{button}, size: geo.NewSize(100, 200)}
	overlay := &bandOverlay{objectTree: objectTree{size: geo.NewSize(100, 200)}, top: 40}
	isButton := func(o core.CanvasObject) bool { return o == button }

	found, _, layer := driver.FindObjectAtPositionMatching(geo.NewPos(50, 20), isButton, []core.CanvasObject{overlay}, content)
	assert.Equal(t, button, found, "above top the content gets it")
	assert.Equal(t, 1, layer)

	overlay.top = 10
	found, _, _ = driver.FindObjectAtPositionMatching(geo.NewPos(50, 20), isButton, []core.CanvasObject{overlay}, content)
	assert.Nil(t, found, "below top the overlay blocks it")

	scroller := &scrollableTree{objectTree: objectTree{size: geo.NewSize(100, 200)}}
	inside := &bandOverlay{objectTree: objectTree{children: []core.CanvasObject{scroller}, size: geo.NewSize(100, 200)}, top: 40}
	all := driver.FindAllObjectsAtPositionMatching(geo.NewPos(50, 20), func(o core.CanvasObject) bool { return o == scroller }, inside)
	assert.Empty(t, all, "above top the overlay's own scrollable is not there")
	all = driver.FindAllObjectsAtPositionMatching(geo.NewPos(50, 60), func(o core.CanvasObject) bool { return o == scroller }, inside)
	assert.Len(t, all, 1, "below top it is")
}

// passOverlay passes input through wherever nothing in it matches, as a
// Toast or a tooltip does.
type passOverlay struct{ objectTree }

func (*passOverlay) PassesInputThrough() bool { return true }

// The overlay stack is searched for scrollables the way taps are resolved
// (#1173): a modal overlay stops the search even where nothing in it
// scrolls, a pass-through one with nothing there lets it continue, and an
// overlay's own scrollable is found wherever it is under the point.
func TestFindAllInOverlaysAt(t *testing.T) {
	size := geo.NewSize(100, 200)
	isScrollable := func(o core.CanvasObject) bool { _, ok := o.(*scrollableTree); return ok }
	at := geo.NewPos(50, 150)

	modal := &objectTree{size: size}
	pass := &passOverlay{objectTree{size: size}}
	found, blocked := driver.FindAllInOverlaysAt(at, isScrollable, []core.CanvasObject{modal})
	assert.Empty(t, found)
	assert.True(t, blocked, "a modal scrim stops the scroll")

	found, blocked = driver.FindAllInOverlaysAt(at, isScrollable, []core.CanvasObject{pass})
	assert.Empty(t, found)
	assert.False(t, blocked, "a pass-through overlay lets it reach the content")

	found, blocked = driver.FindAllInOverlaysAt(at, isScrollable, []core.CanvasObject{modal, pass})
	assert.Empty(t, found)
	assert.True(t, blocked, "a toast above a modal leaves the modal a barrier")

	list := &scrollableTree{objectTree{pos: geo.NewPos(0, 100), size: geo.NewSize(100, 100)}}
	dialog := &objectTree{children: []core.CanvasObject{list}, size: size}
	found, blocked = driver.FindAllInOverlaysAt(at, isScrollable, []core.CanvasObject{dialog, pass})
	assert.False(t, blocked)
	if assert.Len(t, found, 1, "the modal's own list beneath a toast scrolls") {
		assert.Equal(t, list, found[0].Object)
		assert.Equal(t, geo.NewPos(50, 50), found[0].Position)
	}

	band := &bandOverlay{objectTree: objectTree{size: size}, top: 40}
	_, blocked = driver.FindAllInOverlaysAt(geo.NewPos(50, 20), isScrollable, []core.CanvasObject{band})
	assert.False(t, blocked, "above a drawer's top the content scrolls")
	_, blocked = driver.FindAllInOverlaysAt(geo.NewPos(50, 60), isScrollable, []core.CanvasObject{band})
	assert.True(t, blocked, "below it the scrim stops the scroll")
}
