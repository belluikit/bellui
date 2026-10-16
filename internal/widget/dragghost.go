package widget

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
)

// DragGhost is an input-pass-through overlay that paints a drag image following
// the pointer during an in-canvas drag (pre-rebrand tracker #819). It spans the canvas but draws
// only its content at the tracked position, and reports PassesInputThrough so
// the driver's hit-testing falls straight through to the UI beneath — the drag
// dispatch and any drop-target routing must resolve the objects under the
// pointer, never the ghost.
type DragGhost struct {
	Base

	content core.CanvasObject
	pos     geo.Position
}

// NewDragGhost creates a ghost overlay painting content at pos (its top-left).
func NewDragGhost(content core.CanvasObject, pos geo.Position) *DragGhost {
	g := &DragGhost{content: content, pos: pos}
	g.ExtendBaseWidget(g)
	return g
}

// SetPosition moves the ghost content to follow the pointer.
func (g *DragGhost) SetPosition(pos geo.Position) {
	g.pos = pos
	if impl := g.super(); impl != nil {
		cache.Renderer(impl).Layout(g.Size())
		Repaint(impl)
	}
}

// PassesInputThrough marks the ghost as input-transparent for the driver's
// hit-testing (internal/driver.InputPassThrough).
func (g *DragGhost) PassesInputThrough() bool { return true }

// CreateRenderer returns a renderer that lays out the content at the tracked
// position; the overlay root spans the canvas but paints nothing itself.
func (g *DragGhost) CreateRenderer() core.WidgetRenderer {
	return &dragGhostRenderer{ghost: g}
}

type dragGhostRenderer struct {
	ghost *DragGhost
}

func (r *dragGhostRenderer) Layout(geo.Size) {
	r.ghost.content.Resize(r.ghost.content.MinSize())
	r.ghost.content.Move(r.ghost.pos)
}

func (r *dragGhostRenderer) MinSize() geo.Size { return geo.Size{} }

func (r *dragGhostRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.ghost.content}
}

func (r *dragGhostRenderer) Refresh() { Repaint(r.ghost) }
func (r *dragGhostRenderer) Destroy() {}
