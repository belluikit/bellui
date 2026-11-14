package layout

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

type rowWrapLayout struct {
	horizontalPadding float32
	minSize           geo.Size
	verticalPadding   float32
}

// NewRowWrapLayout returns a layout that dynamically arranges objects of similar height
// in rows and wraps them dynamically. Each row is as tall as its tallest object.
// Objects are separated with horizontal and vertical padding.
//
// Since: 1.0
func NewRowWrapLayout() core.Layout {
	p := theme.Padding()
	return &rowWrapLayout{
		horizontalPadding: p,
		verticalPadding:   p,
	}
}

// NewRowWrapLayoutWithCustomPadding returns a new RowWrapLayout instance
// with custom horizontal and inner padding.
//
// Since: 1.0
func NewRowWrapLayoutWithCustomPadding(horizontal, vertical float32) core.Layout {
	return &rowWrapLayout{
		horizontalPadding: horizontal,
		verticalPadding:   vertical,
	}
}

var _ core.Layout = (*rowWrapLayout)(nil)

// MinSize finds the smallest size that satisfies all the child objects.
// For a RowWrapLayout this is initially the width of the widest child
// and the height of the tallest child multiplied by the number of children,
// with appropriate padding between them.
// After Layout() has run it returns the actual min size.
func (l *rowWrapLayout) MinSize(objects []core.CanvasObject) geo.Size {
	if len(objects) == 0 {
		return geo.NewSize(0, 0)
	}
	if !l.minSize.IsZero() {
		return l.minSize
	}
	var maxW, maxH float32
	var objCount int
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		objCount++
		s := o.MinSize()
		maxW = geo.Max(maxW, s.Width)
		maxH = geo.Max(maxH, s.Height)
	}
	return geo.NewSize(maxW, l.minHeight(maxH, objCount))
}

func (l *rowWrapLayout) minHeight(rowHeight float32, rowCount int) float32 {
	return rowHeight*float32(rowCount) + l.verticalPadding*float32(rowCount-1)
}

// Layout is called to pack all child objects into a specified size.
// For RowWrapLayout this will arrange all objects into rows, each as tall as
// its tallest object, and wrap objects into additional rows as needed.
// A core.FitWidther, such as wrapping text, gets its FitWidth, up to the
// container's width, and the height it needs at that width (#1199).
func (l *rowWrapLayout) Layout(objects []core.CanvasObject, containerSize geo.Size) {
	if len(objects) == 0 {
		return
	}
	sizes := make([]geo.Size, len(objects))
	for i, o := range objects {
		if !o.Visible() {
			continue
		}
		size := o.MinSize()
		if give := fitGive(o, size.Width, containerSize.Width); containerSize.Width > 0 && give > 0 {
			size.Width += give
			o.Resize(size)
			size.Height = o.MinSize().Height
		}
		sizes[i] = size
	}

	// Break the objects into rows, then stack the rows.
	row := make([]int, len(objects))
	var rowHeights []float32
	x := float32(0)
	for i, o := range objects {
		if !o.Visible() {
			continue
		}
		if len(rowHeights) == 0 {
			rowHeights = append(rowHeights, 0)
		} else if x+sizes[i].Width+l.horizontalPadding >= containerSize.Width {
			rowHeights = append(rowHeights, 0)
			x = 0
		}
		r := len(rowHeights) - 1
		row[i] = r
		rowHeights[r] = geo.Max(rowHeights[r], sizes[i].Height)
		x += sizes[i].Width + l.horizontalPadding
	}

	var minSize geo.Size
	y := float32(0)
	x = 0
	current := 0
	for i, o := range objects {
		if !o.Visible() {
			continue
		}
		if row[i] != current {
			y += rowHeights[current] + l.verticalPadding
			current = row[i]
			x = 0
		}
		o.Resize(sizes[i])
		o.Move(geo.NewPos(x, y))
		minSize.Width = geo.Max(minSize.Width, x+sizes[i].Width)
		x += sizes[i].Width + l.horizontalPadding
	}
	if len(rowHeights) > 0 {
		minSize.Height = y + rowHeights[current]
	}
	l.minSize = minSize
}
