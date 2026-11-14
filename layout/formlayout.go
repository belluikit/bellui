package layout

import (
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

const formLayoutCols = 2

// Declare conformity with Layout interface
var _ core.Layout = (*formLayout)(nil)

// formLayout is two column grid where each row has a label and a widget.
type formLayout struct{}

// calculateTableSizes calculates the sizes of the table.
// This includes the width of the label column (maximum width of all labels,
// widened toward the widest label's FitWidth when the container has room),
// the width of the content column (maximum width of all content cells and remaining space in container)
// and the total minimum height of the form.
func (f *formLayout) calculateTableSizes(objects []core.CanvasObject, containerWidth float32) (labelWidth float32, contentWidth float32, height float32) {
	if len(objects)%formLayoutCols != 0 {
		return 0, 0, 0
	}

	rows := 0
	labelFit := float32(0)
	innerPadding := theme.InnerPadding()
	for i := 0; i < len(objects); i += formLayoutCols {
		labelCell, contentCell := objects[i], objects[i+1]
		if !labelCell.Visible() && !contentCell.Visible() {
			continue
		}

		// Label column width is the maximum of all labels.
		labelSize := labelCell.MinSize()
		if _, ok := labelCell.(*canvas.Text); ok {
			labelSize.Width += innerPadding * 2
			labelSize.Height += innerPadding * 2
		}
		labelWidth = geo.Max(labelWidth, labelSize.Width)
		if labelCell.Visible() {
			labelFit = geo.Max(labelFit, fitGive(labelCell, 0, 0))
		}

		// Content column width is the maximum of all content items.
		contentSize := contentCell.MinSize()
		if _, ok := contentCell.(*canvas.Text); ok {
			contentSize.Width += innerPadding * 2
			contentSize.Height += innerPadding * 2
		}
		contentWidth = geo.Max(contentWidth, contentSize.Width)

		rowHeight := geo.Max(labelSize.Height, contentSize.Height)
		height += rowHeight
		rows++
	}

	padding := theme.Padding()
	if containerWidth > 0 && labelFit > labelWidth {
		// A FitWidther label, such as wrapping text, widens the label column
		// toward its FitWidth while the content column keeps its MinSize,
		// as an unstretched child of a box does, but to no more than half
		// the width, so a long label cannot squeeze its field (#1202).
		give := labelFit - labelWidth
		w := labelWidth + give*fitKeep(labelWidth+give+padding+contentWidth-containerWidth, give)
		labelWidth = geo.Max(labelWidth, geo.Min(w, (containerWidth-padding)/2))
	}
	contentWidth = geo.Max(contentWidth, containerWidth-labelWidth-padding)
	return labelWidth, contentWidth, height + float32(rows-1)*padding
}

// Layout is called to pack all child objects into a table format with two columns.
func (f *formLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	labelWidth, contentWidth, _ := f.calculateTableSizes(objects, size.Width)

	y := float32(0)
	padding := theme.Padding()
	innerPadding := theme.InnerPadding()

	// Calculate size and position of object. Position and size is returned (instead of calling Move() and Resize()) to make inlineable.
	objectLayout := func(obj core.CanvasObject, offset, width, rowHeight, itemHeight float32) (geo.Position, geo.Size) {
		pos := geo.NewPos(offset, y)
		size := geo.NewSize(width, rowHeight)
		if _, ok := obj.(*canvas.Text); ok {
			pos = pos.AddXY(innerPadding, innerPadding)
			size.Width -= innerPadding * 2
			size.Height = itemHeight
		}

		return pos, size
	}

	remainder := len(objects) % formLayoutCols
	for i := 0; i < len(objects)-remainder; i += formLayoutCols {
		labelCell, contentCell := objects[i], objects[i+1]
		if !labelCell.Visible() && !contentCell.Visible() {
			continue
		}

		labelMin := labelCell.MinSize()
		contentMin := contentCell.MinSize()
		labelHeight := labelMin.Height
		contentHeight := contentMin.Height
		if _, ok := labelCell.(*canvas.Text); ok {
			labelHeight += innerPadding * 2
		}
		if _, ok := contentCell.(*canvas.Text); ok {
			contentHeight += innerPadding * 2
		}
		rowHeight := geo.Max(labelHeight, contentHeight)

		pos, size := objectLayout(labelCell, 0, labelWidth, rowHeight, labelMin.Height)
		labelCell.Move(pos)
		labelCell.Resize(size)

		pos, size = objectLayout(contentCell, labelWidth+padding, contentWidth, rowHeight, contentMin.Height)
		contentCell.Move(pos)
		contentCell.Resize(size)

		y += rowHeight + padding
	}

	// Handle remaining item in the case of uneven number of objects:
	if remainder == 1 {
		lastCell := objects[len(objects)-1]
		lastMin := lastCell.MinSize()
		objectLayout(lastCell, 0, labelWidth, lastMin.Height, lastMin.Height)
	}

	if layoutRTL() {
		MirrorChildren(objects, size.Width)
	}
}

// MinSize finds the smallest size that satisfies all the child objects.
// For a FormLayout this is the width of the widest label and content items and the height is
// the sum of all column children combined with padding between each.
func (f *formLayout) MinSize(objects []core.CanvasObject) geo.Size {
	labelWidth, contentWidth, height := f.calculateTableSizes(objects, 0)
	return geo.Size{
		Width:  labelWidth + contentWidth + theme.Padding(),
		Height: height,
	}
}

// NewFormLayout returns a new FormLayout instance. The label column is as wide
// as its widest label's MinSize, widened toward a [core.FitWidther] label's
// FitWidth, such as wrapping text on one line, to no more than half the width
// and while each field keeps its MinSize.
func NewFormLayout() core.Layout {
	return &formLayout{}
}
