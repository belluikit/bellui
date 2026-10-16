// Package layout defines the various layouts available to Bellui apps.
package layout // import "github.com/belluikit/bellui/layout"

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// Declare conformity with Layout interface
var _ core.Layout = (*stackLayout)(nil)

type stackLayout struct{}

// NewStackLayout returns a new StackLayout instance. Objects are stacked
// on top of each other with later objects on top of those before.
// Having only a single object has no impact as CanvasObjects will
// fill the available space even without a Stack.
//
// Since: 1.0
func NewStackLayout() core.Layout {
	return stackLayout{}
}

// Layout is called to pack all child objects into a specified size.
// For StackLayout this sets all children to the full size passed.
func (m stackLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	topLeft := geo.NewPos(0, 0)
	for _, child := range objects {
		child.Resize(size)
		child.Move(topLeft)
	}
}

// MinSize finds the smallest size that satisfies all the child objects.
// For StackLayout this is determined simply as the MinSize of the largest child.
func (m stackLayout) MinSize(objects []core.CanvasObject) geo.Size {
	minSize := geo.NewSize(0, 0)
	for _, child := range objects {
		if !child.Visible() {
			continue
		}

		minSize = minSize.Max(child.MinSize())
	}

	return minSize
}
