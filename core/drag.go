package core

import "github.com/belluikit/bellui/geo"

// DragImageProvider is an optional extension of a drag source (event.Draggable):
// when a drag begins the driver paints the returned object as a "ghost" that
// follows the pointer, and removes it when the drag ends. Return nil for no
// ghost. It gives in-canvas drag-and-drop a visual affordance without every app
// hand-rolling a following overlay (pre-rebrand tracker #819).
//
// Since: 1.0
type DragImageProvider interface {
	// DragImage returns the object to paint under the pointer during a drag, or
	// nil for no ghost. It is read once when the drag starts.
	DragImage() CanvasObject
}

// DragPayloadProvider is an optional extension of a drag source (event.Draggable):
// the returned value is carried to a [DragTarget] as the in-app drag payload
// (e.g. the palette entry being inserted, the card being moved). It is read once
// when the drag starts. Implementing it (or [DragImageProvider]) also marks the
// drag as an in-canvas DnD drag, which is what enables DragTarget routing — so a
// plain draggable (slider, scrollbar) never triggers drop-target callbacks.
//
// Since: 1.0
type DragPayloadProvider interface {
	// DragPayload returns the value delivered to drop targets, or nil.
	DragPayload() any
}

// DragTargetEvent carries the in-app drag payload and the pointer position to a
// [DragTarget] during an in-canvas drag (pre-rebrand tracker #819). Position is relative to the
// target's top-left; AbsolutePosition is in canvas coordinates.
//
// Since: 1.0
type DragTargetEvent struct {
	AbsolutePosition geo.Position
	Position         geo.Position
	Payload          any
}

// DragTarget is the in-canvas counterpart of bellui.Droppable (which accepts OS
// file drops): a CanvasObject that accepts an in-app widget drag. The driver
// routes enter/over/leave/drop to the deepest DragTarget under the pointer,
// carrying the source-declared payload ([DragPayloadProvider]) — so targets get
// live hover callbacks (to highlight themselves) instead of every app
// re-implementing pointer hit-testing during a drag.
//
// Routing happens only for in-canvas DnD drags — those whose source implements
// [DragPayloadProvider] or [DragImageProvider] — so ordinary draggables never
// reach a DragTarget.
//
// Since: 1.0
type DragTarget interface {
	// DragEnter fires when the pointer enters the target during a drag.
	DragEnter(*DragTargetEvent)
	// DragOver fires on every move while the pointer stays over the target.
	DragOver(*DragTargetEvent)
	// DragLeave fires when the pointer leaves the target (or the drag ends
	// elsewhere) without dropping on it.
	DragLeave()
	// Drop fires when the drag is released over the target.
	Drop(*DragTargetEvent)
}
