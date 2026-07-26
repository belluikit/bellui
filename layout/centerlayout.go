package layout

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// Declare conformity with Layout interface
var _ core.Layout = (*centerLayout)(nil)

type centerLayout struct{}

// NewCenterLayout creates a new CenterLayout instance
func NewCenterLayout() core.Layout {
	return &centerLayout{}
}

// Layout is called to pack all child objects into a specified size.
// For CenterLayout this sets all children to their minimum size, centered within the space.
func (c *centerLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	for _, child := range objects {
		childMin := child.MinSize()
		child.Resize(childMin)
		child.Move(geo.NewPos((size.Width-childMin.Width)/2, (size.Height-childMin.Height)/2))
	}
}

// MinSize finds the smallest size that satisfies all the child objects.
// For CenterLayout this is determined simply as the MinSize of the largest child.
func (c *centerLayout) MinSize(objects []core.CanvasObject) geo.Size {
	minSize := geo.NewSize(0, 0)
	for _, child := range objects {
		if !child.Visible() {
			continue
		}

		minSize = minSize.Max(child.MinSize())
	}

	return minSize
}
