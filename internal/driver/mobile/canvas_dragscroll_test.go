//go:build (ci || android || ios || mobile) && (!windows || !ci) && !wasm

// The Scroll is draggable only where internal/widget/scroller_mobile.go is
// built; on a plain desktop build dragging content does not scroll it, so this
// test carries that file's constraint (#537).

package mobile

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func Test_canvas_Dragged(t *testing.T) {
	dragged := false
	var draggedObj event.Draggable
	scroll := container.NewScroll(widget.NewLabel("Hi\nHi\nHi"))
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(scroll)
	c.Resize(geo.NewSize(40, 24))
	assert.Equal(t, float32(0), scroll.Offset.Y)

	// Drag over the content column, not the right edge: on touch the scrollbar
	// renders permanently at full width (~16px) and occupies the right of this
	// 40px-wide canvas, so a drag there would grab the bar instead of the
	// content we want to fling-scroll.
	c.TapDown(geo.NewPos(10, 3), 0)
	c.TapMove(geo.NewPos(10, 10), 0, func(wid event.Draggable, ev *event.DragEvent) {
		wid.Dragged(ev)
		dragged = true
		draggedObj = wid
	})

	assert.True(t, dragged)
	assert.Equal(t, scroll, draggedObj)
	dragged = false
	c.TapMove(geo.NewPos(10, 5), 0, func(wid event.Draggable, ev *event.DragEvent) {
		wid.Dragged(ev)
		dragged = true
	})
	assert.True(t, dragged)
	assert.Equal(t, geo.NewPos(0, 5), scroll.Offset)
}
