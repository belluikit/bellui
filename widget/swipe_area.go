package widget

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
)

// SwipeDirection is the way a swipe across a SwipeArea went: the way the
// finger moved, on screen, whatever the reading direction.
//
// Since: 1.3
type SwipeDirection string

// The four directions a SwipeArea reports.
//
// Since: 1.3
const (
	SwipeLeft  SwipeDirection = "left"
	SwipeRight SwipeDirection = "right"
	SwipeUp    SwipeDirection = "up"
	SwipeDown  SwipeDirection = "down"
)

// swipeDominance is how much longer a swipe must be along its axis than
// across it, so a diagonal drag reports nothing rather than a guess.
const swipeDominance = 2

var (
	_ core.Widget     = (*SwipeArea)(nil)
	_ event.Draggable = (*SwipeArea)(nil)
	_ event.Tappable  = (*SwipeArea)(nil)
)

// SwipeArea holds one object and reports a swipe across it (#970): a drag
// that goes at least minSwipe, Carousel's distance for a swipe, and mostly
// along one axis. Taps go to the content.
//
// Since: 1.3
type SwipeArea struct {
	BaseWidget

	Content  core.CanvasObject
	OnSwiped func(SwipeDirection) `json:"-"`

	dx, dy float32
}

// NewSwipeArea returns a SwipeArea around content that calls onSwiped.
//
// Since: 1.3
func NewSwipeArea(content core.CanvasObject, onSwiped func(SwipeDirection)) *SwipeArea {
	s := &SwipeArea{Content: content, OnSwiped: onSwiped}
	s.ExtendBaseWidget(s)
	return s
}

// CreateRenderer draws the content, filling the area.
func (s *SwipeArea) CreateRenderer() core.WidgetRenderer {
	s.ExtendBaseWidget(s)
	return NewSimpleRenderer(s.Content)
}

// Dragged adds the drag's movement to the swipe in progress.
func (s *SwipeArea) Dragged(e *event.DragEvent) {
	s.dx += e.Dragged.DX
	s.dy += e.Dragged.DY
}

// DragEnd reports the swipe, if the drag was one, and starts afresh.
func (s *SwipeArea) DragEnd() {
	dx, dy := s.dx, s.dy
	s.dx, s.dy = 0, 0
	if d, ok := swipeOf(dx, dy); ok && s.OnSwiped != nil {
		s.OnSwiped(d)
	}
}

// Tapped passes a tap on to the content.
func (s *SwipeArea) Tapped(e *event.PointEvent) {
	if t, ok := s.Content.(event.Tappable); ok {
		t.Tapped(e)
	}
}

// swipeOf is the direction of a drag that moved dx, dy, if it was a swipe.
func swipeOf(dx, dy float32) (SwipeDirection, bool) {
	ax, ay := abs32(dx), abs32(dy)
	switch {
	case ax >= minSwipe && ax >= swipeDominance*ay:
		if dx < 0 {
			return SwipeLeft, true
		}
		return SwipeRight, true
	case ay >= minSwipe && ay >= swipeDominance*ax:
		if dy < 0 {
			return SwipeUp, true
		}
		return SwipeDown, true
	}
	return "", false
}
