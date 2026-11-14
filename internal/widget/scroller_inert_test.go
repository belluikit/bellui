//go:build ci || android || ios || mobile

package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
)

// An inert Scroll, the one inside a disabled List or tree (#1215), refuses
// the user's scrolling: the wheel, a touch drag of the content, and a tap or
// drag on its bar. It leaves a wheel scroll and a touch drag to the scroll
// around it, and a scroll from code still moves it.
func TestScroll_Inert(t *testing.T) {
	content := canvas.NewRectangle(nil)
	content.SetMinSize(geo.NewSize(100, 1000))
	s := NewVScroll(content)
	inert := true
	s.Inert = func() bool { return inert }
	s.Resize(geo.NewSize(100, 100))
	r := cache.Renderer(s).(*scrollContainerRenderer)
	r.Layout(s.Size())
	moved := 0
	s.OnScrolled = func(geo.Position) { moved++ }
	bar := r.vertArea

	s.Scrolled(&event.ScrollEvent{Scrolled: geo.Delta{DY: -50}})
	assert.False(t, s.TryScrolled(&event.ScrollEvent{Scrolled: geo.Delta{DY: -50}}), "an inert scroll consumed a scroll")
	s.Dragged(&event.DragEvent{Dragged: geo.Delta{DY: -50}})
	assert.False(t, s.ClaimsDrag(geo.Delta{DY: -50}), "an inert scroll claimed a touch drag")
	assert.False(t, s.DragMomentum())
	bar.Tapped(&event.PointEvent{Position: geo.NewPos(5, 90)})
	bar.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: geo.NewPos(5, 90)}})
	bar.bar.Dragged(&event.DragEvent{Dragged: geo.Delta{DY: 50}})
	bar.HoverIn(nil)
	assert.False(t, bar.isMouseIn, "an inert scroll's bar took a hover")
	assert.Zero(t, s.Offset.Y, "an inert scroll moved")
	assert.Zero(t, moved, "OnScrolled fired")

	s.ScrollToOffset(geo.NewPos(0, 50))
	assert.Equal(t, float32(50), s.Offset.Y, "a scroll from code did not move an inert scroll")

	inert = false
	s.Scrolled(&event.ScrollEvent{Scrolled: geo.Delta{DY: -50}})
	assert.Equal(t, float32(100), s.Offset.Y, "a live scroll did not take the wheel")
	bar.Tapped(&event.PointEvent{Position: geo.NewPos(5, 95)})
	assert.Greater(t, s.Offset.Y, float32(100), "a live scroll's bar did not take a tap")
}
