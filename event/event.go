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
	// Since: 3.0
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

	// ScrollSourceTouch indicates scrolling from a direct touch gesture.
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
// On touch devices, this is triggered when the user holds a pointer down
// without moving it for a duration (typically 300ms).
//
// Unlike SecondaryTappable (which is triggered by right-click on desktop),
// LongPressable is specifically for held-touch gestures. A widget can
// implement both interfaces to handle right-click and long-press differently.
type LongPressable interface {
	LongPressed(*PointEvent)
}

// Draggable indicates that a CanvasObject can be dragged.
type Draggable interface {
	Dragged(*DragEvent)
	DragEnd()
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
// giving accessibility/analytics parity with the pointer-tap path (#700) without
// the driver having to guess which keys mean "activate" versus navigation or
// text input.
//
// Focusables that consume Space/Enter as content (text editors) or navigation do
// not implement it. Because it is a capability on the focused object, a widget
// that embeds an activatable base (for example one embedding widget.Button)
// inherits it automatically, and the driver reports the outer focused object.
//
// Since: 2.19
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
// Since: 2.10
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
// Since: 2.10
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
// Since: 2.10
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
type Tabbable interface {
	AcceptsTab() bool
}
