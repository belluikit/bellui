// Package core defines the fundamental interfaces of the Bellui toolkit.
//
// These interfaces form the backbone of every Bellui application:
// [CanvasObject] for anything that can be drawn, [Widget] for interactive
// components, [Canvas] for the rendering surface, [Layout] for arranging
// objects, and [Window] for the top-level UI container.
//
// This package depends only on bellui/geo and bellui/event (both leaf packages),
// making it safe to import from anywhere in the framework.
//
//	import "github.com/belluikit/bellui/core"
package core

import (
	"image"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// CanvasObject describes any graphical object that can be added to a canvas.
// Objects have a size and position that can be controlled through this API.
type CanvasObject interface {
	MinSize() geo.Size
	Move(geo.Position)
	Position() geo.Position
	Resize(geo.Size)
	Size() geo.Size

	Hide()
	Visible() bool
	Show()

	Refresh()
}

// Widget defines the standard behaviours of any widget. This extends
// CanvasObject. A widget behaves in the same basic way but will encapsulate
// many child objects to create the rendered widget.
type Widget interface {
	CanvasObject

	// CreateRenderer returns a new WidgetRenderer for this widget.
	CreateRenderer() WidgetRenderer
}

// WidgetRenderer defines the behaviour of a widget's implementation.
type WidgetRenderer interface {
	Destroy()
	Layout(geo.Size)
	MinSize() geo.Size
	Objects() []CanvasObject
	Refresh()
}

// Layout defines how CanvasObjects may be laid out in a specified Size.
type Layout interface {
	Layout([]CanvasObject, geo.Size)
	MinSize(objects []CanvasObject) geo.Size
}

// OverlayStack is a stack of CanvasObjects used as overlays of a Canvas.
type OverlayStack interface {
	Add(overlay CanvasObject)
	List() []CanvasObject
	Remove(overlay CanvasObject)
	Top() CanvasObject
}

// Disableable describes any CanvasObject that can be disabled.
type Disableable interface {
	Enable()
	Disable()
	Disabled() bool
}

// Transformable describes a CanvasObject that supports a 2D affine transform.
// The transform is applied in addition to the object's position and size.
// When objects in a tree implement Transformable, their transforms compose:
// a child's world transform is parent.worldTransform * Translate(child.Position()) * child.Transform().
type Transformable interface {
	Transform() geo.Matrix
	SetTransform(geo.Matrix)
}

// Shortcut describes a keyboard shortcut (e.g. Ctrl+C).
type Shortcut interface {
	ShortcutName() string
}

// Canvas defines a graphical canvas to which a CanvasObject can be added.
// Each canvas has a scale which is automatically applied during the render process.
type Canvas interface {
	Content() CanvasObject
	SetContent(CanvasObject)
	Refresh(CanvasObject)

	Focus(event.Focusable)
	FocusNext()
	FocusPrevious()
	Unfocus()
	Focused() event.Focusable

	Size() geo.Size
	Scale() float32

	Overlays() OverlayStack

	OnTypedRune() func(rune)
	SetOnTypedRune(func(rune))
	OnTypedKey() func(*event.KeyEvent)
	SetOnTypedKey(func(*event.KeyEvent))
	AddShortcut(shortcut Shortcut, handler func(shortcut Shortcut))
	RemoveShortcut(shortcut Shortcut)

	Capture() image.Image

	PixelCoordinateForPosition(geo.Position) (int, int)
	InteractiveArea() (geo.Position, geo.Size)
}
