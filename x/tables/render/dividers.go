package render

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// DividerLayer manages horizontal and vertical grid line separators.
type DividerLayer struct {
	color     color.Color
	thickness float32
	hLines    []*canvas.Rectangle
	vLines    []*canvas.Rectangle
}

// NewDividerLayer creates a new divider layer.
func NewDividerLayer(c color.Color, thickness float32) *DividerLayer {
	return &DividerLayer{
		color:     c,
		thickness: thickness,
	}
}

// Update repositions dividers for the visible rows and columns.
func (d *DividerLayer) Update(vp *Viewport, offsetX, offsetY float32, viewSize geo.Size) {
	visRows := vp.VisibleRows
	visCols := vp.VisibleCols

	// Horizontal dividers (row separators)
	hCount := visRows.Max - visRows.Min + 1
	for len(d.hLines) < hCount {
		r := canvas.NewRectangle(d.color)
		d.hLines = append(d.hLines, r)
	}
	for i := 0; i <= visRows.Max-visRows.Min; i++ {
		row := visRows.Min + i
		y, rowH := vp.RowY(row)
		line := d.hLines[i]
		line.FillColor = d.color
		line.Move(geo.NewPos(0, y+rowH-offsetY))
		line.Resize(geo.NewSize(viewSize.Width, d.thickness))
		line.Show()
	}
	for i := hCount; i < len(d.hLines); i++ {
		d.hLines[i].Hide()
	}

	// Vertical dividers (column separators)
	vCount := visCols.Max - visCols.Min + 1
	for len(d.vLines) < vCount {
		r := canvas.NewRectangle(d.color)
		d.vLines = append(d.vLines, r)
	}
	for i := 0; i <= visCols.Max-visCols.Min; i++ {
		col := visCols.Min + i
		x, colW := vp.ColX(col)
		line := d.vLines[i]
		line.FillColor = d.color
		line.Move(geo.NewPos(x+colW-offsetX, 0))
		line.Resize(geo.NewSize(d.thickness, viewSize.Height))
		line.Show()
	}
	for i := vCount; i < len(d.vLines); i++ {
		d.vLines[i].Hide()
	}
}

// Objects returns all divider CanvasObjects.
func (d *DividerLayer) Objects() []core.CanvasObject {
	objs := make([]core.CanvasObject, 0, len(d.hLines)+len(d.vLines))
	for _, l := range d.hLines {
		objs = append(objs, l)
	}
	for _, l := range d.vLines {
		objs = append(objs, l)
	}
	return objs
}

// SetColor updates the divider color.
func (d *DividerLayer) SetColor(c color.Color) {
	d.color = c
}
