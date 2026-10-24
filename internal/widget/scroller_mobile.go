//go:build ci || android || ios || mobile

package widget

import "github.com/belluikit/bellui/event"

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
