// Package scroll defines an event for pointer scrolling on a touch device: a
// mouse wheel, or a touchpad's two-finger scroll, as Android and iPadOS
// report them (#1510). A finger dragging the screen is a touch event, not
// this.
package scroll // import "github.com/belluikit/bellui/internal/driver/mobile/event/scroll"

// Event is one pointer scroll step, or the end of a touchpad gesture.
type Event struct {
	// X and Y are the pointer's location, in pixels.
	X, Y float32

	// DX and DY are how far to scroll, in pixels, in bellui's wheel
	// convention: a positive DY scrolls toward the top, as a wheel turned
	// away from the user does, and a positive DX toward the left.
	DX, DY float32

	// Touchpad is set for a touchpad gesture's steps and end, and clear for
	// a mouse wheel.
	Touchpad bool

	// Phase is where the step stands in a touchpad gesture.
	Phase Phase
}

// Phase is where a scroll stands in a touchpad gesture.
type Phase byte

const (
	// PhaseNone is a mouse wheel step, outside any gesture.
	PhaseNone Phase = iota
	// PhaseBegan is the fingers touching down, before they move.
	PhaseBegan
	// PhaseChanged is the fingers moving.
	PhaseChanged
	// PhaseEnded is the fingers lifting, or the system cancelling the
	// gesture. It carries no movement.
	PhaseEnded
)
