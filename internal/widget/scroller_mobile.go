//go:build ci || android || ios || mobile

package widget

import (
	"math"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// DragEnd will stop scrolling on mobile has stopped
func (s *Scroll) DragEnd() {
}

// DragMomentum makes a released touch drag of the content coast, seeded from
// the finger's speed at release (event.MomentumDraggable, #751). A Scroll that
// scrolls in no direction has nothing to coast.
func (s *Scroll) DragMomentum() bool {
	return s.Direction != ScrollNone
}

// Dragged will scroll on any drag - bar or otherwise - for mobile
func (s *Scroll) Dragged(e *event.DragEvent) {
	if s.updateOffset(e.Dragged.DX, e.Dragged.DY) {
		s.refreshWithoutOffsetUpdate()
	}
}

// ClaimsDrag takes a touch drag that moves the content along the axis the
// drag mostly follows (event.DragClaimer, #662). A drag along an axis this
// Scroll does not scroll, or one that pushes against the end its content
// already reached, is left to the Draggable around it: a scroll nested in a
// page hands the page a drag it cannot follow.
func (s *Scroll) ClaimsDrag(d geo.Delta) bool {
	if s.Content == nil || s.Direction == ScrollNone {
		return false
	}
	if math.Abs(float64(d.DX)) > math.Abs(float64(d.DY)) {
		if s.Direction == ScrollVerticalOnly {
			return false
		}
		d.DY = 0
	} else {
		if s.Direction == ScrollHorizontalOnly {
			return false
		}
		d.DX = 0
	}
	size, min := s.Size(), s.Content.MinSize()
	return computeOffset(s.Offset.X, -d.DX, size.Width, min.Width) != s.Offset.X ||
		computeOffset(s.Offset.Y, -d.DY, size.Height, min.Height) != s.Offset.Y
}
