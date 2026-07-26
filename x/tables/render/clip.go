package render

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// ClipRegion wraps content in a clipping container for pinned column/row regions.
// Content that exceeds the clip bounds is not rendered.
type ClipRegion struct {
	Container *bellui.Container
	bg        *canvas.Rectangle
}

// NewClipRegion creates a clipped container.
func NewClipRegion() *ClipRegion {
	bg := canvas.NewRectangle(nil)
	c := container.NewWithoutLayout(bg)
	return &ClipRegion{
		Container: c,
		bg:        bg,
	}
}

// SetContent replaces the clip region's content objects.
func (cr *ClipRegion) SetContent(objects ...core.CanvasObject) {
	all := make([]core.CanvasObject, 0, len(objects)+1)
	all = append(all, cr.bg)
	all = append(all, objects...)
	cr.Container.Objects = all
	cr.Container.Refresh()
}

// Resize sets the size of the clip region.
func (cr *ClipRegion) Resize(size geo.Size) {
	cr.Container.Resize(size)
	cr.bg.Resize(size)
}

// Move positions the clip region.
func (cr *ClipRegion) Move(pos geo.Position) {
	cr.Container.Move(pos)
}
