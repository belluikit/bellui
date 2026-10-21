package bellui

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/layoutgen"
	"github.com/belluikit/bellui/internal/paintgen"
)

// Declare conformity to [core.CanvasObject]
var _ core.CanvasObject = (*Container)(nil)

// Container is a [core.CanvasObject] that contains a collection of child objects.
// The layout of the children is set by the specified Layout.
type Container struct {
	size     geo.Size     // The current size of the Container
	position geo.Position // The current position of the Container
	Hidden   bool         // Is this Container hidden

	Layout       core.Layout         // The Layout algorithm for arranging child [core.CanvasObject]s
	Objects      []core.CanvasObject // The set of [core.CanvasObject]s this container holds
	ClipToBounds bool                // When true, children are clipped to the container's bounds
}

// Add appends the specified object to the items this container manages.
// It is safe to call from any goroutine.
//
// Since: 1.0
func (c *Container) Add(add core.CanvasObject) {
	RunOnMain(func() {
		if add == nil {
			return
		}
		c.Objects = append(c.Objects, add)
		c.layout()
		placed(c)
	})
}

// Hide sets this container, and all its children, to be not visible.
func (c *Container) Hide() {
	if c.Hidden {
		return
	}

	c.Hidden = true
	layoutgen.Changed()
	repaint(c)
}

// MinSize calculates the minimum size of c.
// This is delegated to the [Container.Layout], if specified, otherwise it will be calculated.
func (c *Container) MinSize() geo.Size {
	if c.Layout != nil {
		return c.Layout.MinSize(c.Objects)
	}

	minSize := geo.NewSize(1, 1)
	for _, child := range c.Objects {
		minSize = minSize.Max(child.MinSize())
	}

	return minSize
}

// Move the container (and all its children) to a new position, relative to its parent.
func (c *Container) Move(pos geo.Position) {
	c.position = pos
	repaint(c)
}

// Position gets the current position of c relative to its parent.
func (c *Container) Position() geo.Position {
	return c.position
}

// Refresh causes this object to be redrawn in its current state
func (c *Container) Refresh() {
	c.layout()

	for _, child := range c.Objects {
		child.Refresh()
	}

	// this is basically just canvas.Refresh(c) without the package loop
	o := CurrentApp().Driver().CanvasForObject(c)
	if o == nil {
		return
	}
	o.Refresh(c)
}

// Remove updates the contents of this container to no longer include the specified object.
// This method is not intended to be used inside a loop, to remove all the elements.
// It is much more efficient to call [Container.RemoveAll) instead.
func (c *Container) Remove(rem core.CanvasObject) {
	if len(c.Objects) == 0 {
		return
	}

	for i, o := range c.Objects {
		if o != rem {
			continue
		}
		copy(c.Objects[i:], c.Objects[i+1:])
		c.Objects[len(c.Objects)-1] = nil
		c.Objects = c.Objects[:len(c.Objects)-1]
		c.layout()
		placed(c)
		return
	}
}

// RemoveAll updates the contents of this container to no longer include any objects.
//
// Since: 1.0
func (c *Container) RemoveAll() {
	c.Objects = nil
	c.layout()
	placed(c)
}

// Resize sets a new size for c.
func (c *Container) Resize(size geo.Size) {
	if c.size == size {
		return
	}

	c.size = size
	c.layout()
	placed(c)
}

// Show sets this container, and all its children, to be visible.
func (c *Container) Show() {
	if !c.Hidden {
		return
	}

	c.Hidden = false
	// A container hidden from the start was never walked, so no canvas knows
	// it: the next min-size pass must walk everything to lay it out (#487),
	// and the next partial repaint everything to find where it shows (#599).
	layoutgen.Changed()
	paintgen.Changed()
	repaint(c)
}

// Size returns the current size c.
func (c *Container) Size() geo.Size {
	return c.size
}

// Visible returns true if the container is currently visible, false otherwise.
func (c *Container) Visible() bool {
	return !c.Hidden
}

func (c *Container) layout() {
	if c.Layout == nil {
		return
	}

	c.Layout.Layout(c.Objects, c.size)
}

// repaint instructs the containing canvas to redraw, even if nothing changed.
// This method is a duplicate of what is in `canvas/canvas.go` to avoid a dependency loop or public API.
func repaint(obj *Container) {
	app := CurrentApp()
	if app == nil || app.Driver() == nil {
		return
	}

	c := app.Driver().CanvasForObject(obj)
	if c != nil {
		// Showing or hiding changes the layout around obj: let the min-size
		// pass know, as a refresh would (#487).
		if m, ok := c.(interface{ MarkMinSizeDirty(core.CanvasObject) }); ok {
			m.MarkMinSizeDirty(obj)
		}
		if m, ok := c.(interface{ MarkPlacementDirty(core.CanvasObject) }); ok {
			m.MarkPlacementDirty(obj)
		}
		if paint, ok := c.(interface{ SetDirty() }); ok {
			paint.SetDirty()
		}
	}
}

// placed tells the canvas showing c that c was resized or its objects
// changed, which a partial repaint learns of from this alone (#599).
func placed(c *Container) {
	app := CurrentApp()
	if app == nil || app.Driver() == nil {
		return
	}
	if cv := app.Driver().CanvasForObject(c); cv != nil {
		if m, ok := cv.(interface{ MarkPlacementDirty(core.CanvasObject) }); ok {
			m.MarkPlacementDirty(c)
		}
	}
}
