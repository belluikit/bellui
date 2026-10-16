package embedded

import "github.com/belluikit/bellui/geo"

// TouchDownEvent is for indicating that an embedded device touch screen or pointing device was pressed.
//
// Since: 1.0
type TouchDownEvent struct {
	Position geo.Position
	ID       int
}

func (t *TouchDownEvent) isEvent() {}

// TouchMoveEvent is for indicating that an embedded device touch screen or pointing device was moved whilst being pressed.
//
// Since: 1.0
type TouchMoveEvent struct {
	Position geo.Position
	ID       int
}

func (t *TouchMoveEvent) isEvent() {}

// TouchUpEvent is for indicating that an embedded device touch screen or pointing device was released.
//
// Since: 1.0
type TouchUpEvent struct {
	Position geo.Position
	ID       int
}

func (t *TouchUpEvent) isEvent() {}
