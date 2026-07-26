package interact

import "github.com/belluikit/bellui/geo"

// BrushConfig configures brush selection behavior.
type BrushConfig struct {
	Enabled bool
}

// BrushState tracks an in-progress brush selection.
type BrushState struct {
	Active     bool         // true while the user is dragging
	StartPos   geo.Position // position where the drag began
	CurrentPos geo.Position // current drag position
}

// BrushRect returns the normalized rectangle (top-left, size) of the current selection.
func (b *BrushState) BrushRect() (geo.Position, geo.Size) {
	x1, x2 := b.StartPos.X, b.CurrentPos.X
	y1, y2 := b.StartPos.Y, b.CurrentPos.Y
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	return geo.NewPos(x1, y1), geo.NewSize(x2-x1, y2-y1)
}

// FindSelectedPoints returns all hit tester points within the brush rectangle.
func FindSelectedPoints(ht *HitTester, brushPos geo.Position, brushSize geo.Size) []BrushSelectedPoint {
	var selected []BrushSelectedPoint
	bx1 := brushPos.X
	by1 := brushPos.Y
	bx2 := bx1 + brushSize.Width
	by2 := by1 + brushSize.Height

	for _, e := range ht.entries {
		for _, p := range e.points {
			if p.pixelPos.X >= bx1 && p.pixelPos.X <= bx2 &&
				p.pixelPos.Y >= by1 && p.pixelPos.Y <= by2 {
				selected = append(selected, BrushSelectedPoint{
					SeriesIndex: e.seriesIndex,
					DataIndex:   p.dataIndex,
					Value:       p.value,
				})
			}
		}
	}

	return selected
}
