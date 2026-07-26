package mobile

import "github.com/belluikit/bellui/event"

// TouchEvent contains data relating to mobile touch events
type TouchEvent struct {
	event.PointEvent
}

// Touchable represents mobile touch events that can be sent to CanvasObjects
type Touchable interface {
	TouchDown(*TouchEvent)
	TouchUp(*TouchEvent)
	TouchCancel(*TouchEvent)
}
