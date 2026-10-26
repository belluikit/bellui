// Package event defines input event types and behavioral interfaces for
// canvas objects that respond to user interaction.
//
// This package has no dependency on the root bellui package, making it safe
// to import from any level of the framework without circular dependency.
//
//	import "github.com/belluikit/bellui/event"
//
//	type MyButton struct { widget.BaseWidget }
//	func (b *MyButton) Tapped(ev *event.PointEvent) { /* handle tap */ }
package event

import "github.com/belluikit/bellui/geo"

// --- Event types ---

// HardwareKey contains information associated with physical key events.
type HardwareKey struct {
	// ScanCode represents a hardware ID for (normally desktop) keyboard events.
	ScanCode int
}

// KeyEvent describes a keyboard input event.
type KeyEvent struct {
	// Name describes the keyboard event that is consistent across platforms.
	Name KeyName
	// Physical is a platform specific field that reports hardware information.
	Physical HardwareKey
	// Modifier reports the modifier keys held when the event was generated,
	// as a [KeyModifier] bitmask. The zero value means no modifiers. It lets
	// key handlers (TypedKey, Canvas.SetOnTypedKey) distinguish e.g. Shift+F11
	// from F11 without registering a shortcut. The desktop and web drivers
	// populate it from the native key event; the mobile driver reports what
	// the platform delivers for hardware keyboards; drivers without modifier
	// information leave it zero.
	//
	// Note that on desktop, Control/Alt/Super combinations are routed to the
	// shortcut system instead of TypedKey (unchanged behavior) — only
	// unmodified and Shift-modified keys reach TypedKey there. All
	// combinations are visible with modifiers on the KeyDown/KeyUp events of
	// a widget implementing the desktop driver's Keyable. For the window-wide
	// modifier state outside a key event, see the desktop driver's
	// KeyModifierDriver interface.
	//
	// Since: 1.0
	Modifier KeyModifier
}

// PointEvent describes a pointer input event. The position is relative to the
// top-left of the CanvasObject this is triggered on.
type PointEvent struct {
	AbsolutePosition geo.Position // The absolute position of the event
	Position         geo.Position // The relative position of the event
}

// ScrollSource identifies the input device that generated a scroll event.
type ScrollSource int

const (
	// ScrollSourceUnknown indicates the scroll source is not known.
	ScrollSourceUnknown ScrollSource = iota

	// ScrollSourceMouseWheel indicates scrolling from a mouse scroll wheel.
	ScrollSourceMouseWheel

	// ScrollSourceTouchpad indicates scrolling from a touchpad gesture.
	ScrollSourceTouchpad

	// ScrollSourceTouch indicates scrolling from a direct touch gesture — a
	// finger dragging the content itself. Unlike the pointer sources it is
	// direct manipulation, so the delta is direction-absolute: a scroller
	// applies it raw, without the pointer conventions (right-to-left flip,
	// vertical-to-horizontal axis redirect) that a wheel or touchpad delta
	// goes through. Synthesized scrolls that mean "advance a page" rather
	// than "the content follows this finger" want a pointer source instead.
	ScrollSourceTouch
)

// ScrollEvent defines the parameters of a pointer or other scroll event.
type ScrollEvent struct {
	PointEvent
	Scrolled geo.Delta
	Source   ScrollSource
}

// DragEvent defines the parameters of a pointer or other drag event.
type DragEvent struct {
	PointEvent
	Dragged geo.Delta
}

// HoverEvent describes a pointer hovering over or entering/leaving a
// canvas object. Unlike desktop.MouseEvent it carries only position
// information — no button or modifier state.
type HoverEvent struct {
	PointEvent
}

// --- Behavioral interfaces ---

// Tappable describes any CanvasObject that can be tapped.
type Tappable interface {
	Tapped(*PointEvent)
}

// DoubleTappable describes any CanvasObject that can be double tapped.
type DoubleTappable interface {
	DoubleTapped(*PointEvent)
}

// SecondaryTappable describes a CanvasObject that can be right-clicked or long-tapped.
type SecondaryTappable interface {
	TappedSecondary(*PointEvent)
}

// LongPressable describes a CanvasObject that responds to long-press gestures.
// It is triggered when the user holds a pointer down without moving it for a
// duration (typically 300ms): a held finger on touch devices, and a held
// primary mouse button on desktop (#266), where the gesture fires while the
// button is still down and its release delivers no Tapped.
//
// Unlike SecondaryTappable (which is triggered by right-click on desktop),
// LongPressable is the press-and-hold gesture. A widget can implement both
// interfaces to handle right-click and long-press differently.
//
// A widget that may decline the gesture must answer the press some other way
// rather than ignore it: the drivers have already withheld the Tapped that
// would have followed.
type LongPressable interface {
	LongPressed(*PointEvent)
}

// Draggable indicates that a CanvasObject can be dragged.
type Draggable interface {
	Dragged(*DragEvent)
	DragEnd()
}

// MomentumDraggable is a Draggable that keeps moving after a touch drag is let
// go at speed, the way flung content does. It is optional: a Draggable that
// does not implement it, or whose DragMomentum reports false, gets DragEnd
// where the finger lifted.
//
// On the mobile and embedded drivers, a touch drag of a MomentumDraggable
// released while the finger was still moving carries on: the object gets
// decaying Dragged events, seeded from the finger's speed at release, and then
// DragEnd. A finger held still before lifting does not coast, and the next
// touch stops a coast at once. That touch only stops it: it taps, double-taps
// and long-presses nothing, though it can drag on. Desktop drags, mouse or
// touch, never coast.
//
// Direct manipulation — a slider thumb, a divider, a selection handle, a
// drag-reorder — must not opt in: what the user let go of has to stay there.
// Scroll containers do.
type MomentumDraggable interface {
	Draggable
	// DragMomentum reports whether the drag being released should coast. It
	// is asked once per release, on the main goroutine, before any coast
	// step or DragEnd, so the answer may depend on what the drag is doing
	// (panning but not resizing), and a type embedding one can return false.
	DragMomentum() bool
}

// PinchEvent describes one step of a two-finger pinch. Position and
// AbsolutePosition are the focus, the midpoint between the fingers, relative
// to the receiving object and to the canvas. Scale is relative to the previous
// event of the same pinch: above 1 the fingers moved apart (zoom in), below 1
// they moved together, and exactly 1 when they moved without spreading or
// closing (the focus moved). Multiply successive values for the total.
//
// Since: 1.2
type PinchEvent struct {
	PointEvent
	Scale float32
}

// Pinchable is implemented by objects that respond to a two-finger pinch, such
// as zoomable canvases. Two fingers whose distance changes at least as much as
// their midpoint moves are a pinch when both fingers are on a Pinchable as the
// second one lands: the Pinchable under their midpoint, if both fingers are
// inside its bounds, receives Pinched for each move and PinchEnded once after
// the last one, when either finger lifts. Two fingers on either side of a
// Pinchable, or only one of them on it, do not pinch it; each keeps its own
// tap or drag. Two fingers moving together are a pan instead, delivered as
// touch-sourced scrolling. Where a driver has no touch scrolling, two fingers
// on a Pinchable always pinch, and a move that keeps their distance reports
// the new focus with Scale 1. Neither finger of a two-finger gesture produces
// taps or drags.
//
// Since: 1.2
type Pinchable interface {
	Pinched(*PinchEvent)
	PinchEnded()
}

// Focusable describes any CanvasObject that can respond to being focused.
type Focusable interface {
	FocusGained()
	FocusLost()
	TypedRune(rune)
	TypedKey(*KeyEvent)
}

// Activatable is an optional capability of a [Focusable] control that a keyboard
// key (typically Space or Enter) activates the same way a pointer tap does — a
// button, checkbox, switch, menu item, list row, etc. It lets the driver fire
// the central activation signal (core.NotifyActivation) for keyboard activation,
// giving accessibility/analytics parity with the pointer-tap path (pre-rebrand tracker #700) without
// the driver having to guess which keys mean "activate" versus navigation or
// text input.
//
// Focusables that consume Space/Enter as content (text editors) or navigation do
// not implement it. Because it is a capability on the focused object, a widget
// that embeds an activatable base (for example one embedding widget.Button)
// inherits it automatically, and the driver reports the outer focused object.
//
// Since: 1.0
type Activatable interface {
	Focusable
	// ActivatedByKey reports whether the given key event activates this object
	// (rather than being consumed as navigation or text). The driver calls it for
	// the focused object on each key event and, when it returns true, fires the
	// activation signal just before forwarding the key to [Focusable.TypedKey].
	ActivatedByKey(*KeyEvent) bool
}

// PreeditEvent describes in-progress IME composition (marked/preedit) text.
// During composition an input method shows tentative text inline before it is
// committed; the final committed text arrives separately via
// [Composable.ComposeCommit].
//
// Since: 1.0
type PreeditEvent struct {
	// Text is the composition (preedit) text to display inline. An empty
	// string clears any visible preedit without committing.
	Text string
	// CursorPos is the caret position within Text, measured in runes.
	CursorPos int
}

// Composable is an optional capability of a [Focusable] that supports IME
// (input method editor) composition, allowing CJK and other users to enter
// text via a preedit/candidate workflow. Drivers detect support via a type
// assertion on the focused object.
//
// Since: 1.0
type Composable interface {
	Focusable
	// Compose sets or replaces the inline preedit text shown at the caret. It
	// must not alter the committed value, fire change callbacks, or affect undo.
	Compose(*PreeditEvent)
	// ComposeCommit finalizes the composition, inserting text as if it had been
	// typed (updating the value, undo history and change callbacks).
	ComposeCommit(string)
	// ComposeCancel discards any in-progress composition without committing.
	ComposeCancel()
}

// PreeditPositioner is an optional capability of a [Composable] that reports the
// caret rectangle, in its own content coordinates, so a driver can position the
// IME candidate window near the text cursor.
//
// Since: 1.0
type PreeditPositioner interface {
	// PreeditCursorRectangle returns the caret rectangle in the object's content
	// coordinate space. A zero rectangle means the position is unknown.
	PreeditCursorRectangle() geo.Rectangle
}

// Scrollable describes any CanvasObject that can be scrolled.
type Scrollable interface {
	Scrolled(*ScrollEvent)
}

// Note: Shortcutable is defined in the root bellui package (shortcut.go)
// because it references core.Shortcut which cannot be imported here.

// Hoverable describes any CanvasObject that wants to know when a pointer
// hovers over it.
type Hoverable interface {
	// HoverIn is called when a pointer enters the element.
	HoverIn(*HoverEvent)
	// HoverMoved is called when a pointer moves within the element.
	HoverMoved(*HoverEvent)
	// HoverOut is called when a pointer leaves the element.
	HoverOut()
}

// Tabbable describes any object that needs to accept Tab key presses.
//
// While an object whose AcceptsTab reports true is focused, plain Tab and
// Shift+Tab reach it instead of moving focus. So that keyboard users are never
// trapped, two key sequences still move focus out of it: Ctrl+Tab /
// Ctrl+Shift+Tab, and Escape followed by Tab / Shift+Tab. Browsers keep
// Ctrl+Tab for themselves, so on the web only the second one works.
type Tabbable interface {
	AcceptsTab() bool
}

// EscapeDismissible describes an overlay that closes when the user presses
// Escape. Pop-ups, menus and dialogs implement it, so backing out of whatever
// is on top works everywhere without each one wiring up its own key handler.
//
// Overlays are asked innermost-first. An implementing overlay that declines
// passes the key to the overlay beneath it; the first to report true consumes
// the key. An overlay that does not implement the interface ends the search
// with the key unconsumed — it is assumed to handle Escape through the normal
// focus path, the way menus and completion lists do.
//
// Since: 1.0
type EscapeDismissible interface {
	// DismissOnEscape closes this overlay and reports whether it did. An
	// overlay that owns its own lifetime — a snackbar counting down to its own
	// dismissal — returns false and leaves the key to whatever is beneath it.
	DismissOnEscape() bool
}

// EscapeHandler describes a [Focusable] that wants the Escape key for itself
// rather than letting it close the surrounding pop-up or dialog.
//
// Escape means "back out of the innermost thing", and a widget can be that
// innermost thing without being an overlay: a text field mid-IME-composition,
// a cell being edited in place, a picker holding a half-typed value. Such a
// widget reports true while it is in that state and false the rest of the
// time, so Escape falls through to the dialog around it once there is nothing
// left to back out of.
//
// A focusable that does not implement this never blocks the key.
//
// Since: 1.0
type EscapeHandler interface {
	HandlesEscape() bool
}
