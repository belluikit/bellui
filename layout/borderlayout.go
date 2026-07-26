package layout

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// Declare conformity with Layout interface
var _ core.Layout = (*borderLayout)(nil)

type borderLayout struct {
	top, bottom, left, right core.CanvasObject
}

// NewBorderLayout creates a new BorderLayout instance with top, bottom, left
// and right objects set. All other items in the container will fill the remaining space in the middle.
// Multiple extra items will be stacked in the specified order as a Stack container.
func NewBorderLayout(top, bottom, left, right core.CanvasObject) core.Layout {
	return &borderLayout{top, bottom, left, right}
}

// Layout is called to pack all child objects into a specified size.
// For BorderLayout this arranges the top, bottom, left and right widgets at
// the sides and any remaining widgets are maximised in the middle space.
func (b *borderLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	padding := theme.Padding()
	var topSize, bottomSize, leftSize, rightSize geo.Size
	if b.top != nil && b.top.Visible() {
		topHeight := b.top.MinSize().Height
		b.top.Resize(geo.NewSize(size.Width, topHeight))
		b.top.Move(geo.NewPos(0, 0))
		topSize = geo.NewSize(size.Width, topHeight+padding)
	}
	if b.bottom != nil && b.bottom.Visible() {
		bottomHeight := b.bottom.MinSize().Height
		b.bottom.Resize(geo.NewSize(size.Width, bottomHeight))
		b.bottom.Move(geo.NewPos(0, size.Height-bottomHeight))
		bottomSize = geo.NewSize(size.Width, bottomHeight+padding)
	}
	if b.left != nil && b.left.Visible() {
		leftWidth := b.left.MinSize().Width
		b.left.Resize(geo.NewSize(leftWidth, size.Height-topSize.Height-bottomSize.Height))
		b.left.Move(geo.NewPos(0, topSize.Height))
		leftSize = geo.NewSize(leftWidth+padding, size.Height-topSize.Height-bottomSize.Height)
	}
	if b.right != nil && b.right.Visible() {
		rightWidth := b.right.MinSize().Width
		b.right.Resize(geo.NewSize(rightWidth, size.Height-topSize.Height-bottomSize.Height))
		b.right.Move(geo.NewPos(size.Width-rightWidth, topSize.Height))
		rightSize = geo.NewSize(rightWidth+padding, size.Height-topSize.Height-bottomSize.Height)
	}

	middleSize := geo.NewSize(size.Width-leftSize.Width-rightSize.Width, size.Height-topSize.Height-bottomSize.Height)
	middlePos := geo.NewPos(leftSize.Width, topSize.Height)
	for _, child := range objects {
		if !child.Visible() {
			continue
		}

		if child != b.top && child != b.bottom && child != b.left && child != b.right {
			child.Resize(middleSize)
			child.Move(middlePos)
		}
	}

	if layoutRTL() {
		MirrorChildren(objects, size.Width)
	}
}

// MinSize finds the smallest size that satisfies all the child objects.
// For BorderLayout this is determined by the MinSize height of the top and
// plus the MinSize width of the left and right, plus any padding needed.
// This is then added to the union of the MinSize for any remaining content.
func (b *borderLayout) MinSize(objects []core.CanvasObject) geo.Size {
	minSize := geo.NewSize(0, 0)
	for _, child := range objects {
		if !child.Visible() {
			continue
		}

		if child != b.top && child != b.bottom && child != b.left && child != b.right {
			minSize = minSize.Max(child.MinSize())
		}
	}

	padding := theme.Padding()

	if b.left != nil && b.left.Visible() {
		leftMin := b.left.MinSize()
		minHeight := geo.Max(minSize.Height, leftMin.Height)
		minSize = geo.NewSize(minSize.Width+leftMin.Width+padding, minHeight)
	}
	if b.right != nil && b.right.Visible() {
		rightMin := b.right.MinSize()
		minHeight := geo.Max(minSize.Height, rightMin.Height)
		minSize = geo.NewSize(minSize.Width+rightMin.Width+padding, minHeight)
	}

	if b.top != nil && b.top.Visible() {
		topMin := b.top.MinSize()
		minWidth := geo.Max(minSize.Width, topMin.Width)
		minSize = geo.NewSize(minWidth, minSize.Height+topMin.Height+padding)
	}
	if b.bottom != nil && b.bottom.Visible() {
		bottomMin := b.bottom.MinSize()
		minWidth := geo.Max(minSize.Width, bottomMin.Width)
		minSize = geo.NewSize(minWidth, minSize.Height+bottomMin.Height+padding)
	}

	return minSize
}
