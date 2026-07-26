package desktop

import "github.com/belluikit/bellui/event"

// MouseButton represents a single button in a desktop MouseEvent
type MouseButton int

const (
	// MouseButtonPrimary is the most common mouse button - on some systems the only one.
	// This will normally be on the left side of a mouse.
	//
	// Since: 2.0
	MouseButtonPrimary MouseButton = 1 << iota

	// MouseButtonSecondary is the secondary button on most mouse input devices.
	// This will normally be on the right side of a mouse.
	//
	// Since: 2.0
	MouseButtonSecondary

	// MouseButtonTertiary is the middle button on the mouse, assuming it has one.
	//
	// Since: 2.0
	MouseButtonTertiary
)

// MouseEvent contains data relating to desktop mouse events
type MouseEvent struct {
	event.PointEvent
	Button   MouseButton
	Modifier event.KeyModifier
}

// Mouseable represents desktop mouse events that can be sent to CanvasObjects
type Mouseable interface {
	MouseDown(*MouseEvent)
	MouseUp(*MouseEvent)
}
