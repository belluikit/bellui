package interact

import (
	"fmt"
	"image/color"
	"math"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/x/charts/coord"
)

// DrawingToolMode identifies the active drawing tool.
type DrawingToolMode int

const (
	DrawingToolNone           DrawingToolMode = iota
	DrawingToolHLine                          // horizontal price level
	DrawingToolVLine                          // vertical time marker
	DrawingToolTrendLine                      // two-point line segment
	DrawingToolRay                            // half-infinite line from start through direction
	DrawingToolRectangle                      // rectangular zone
	DrawingToolChannel                        // two parallel trend lines
	DrawingToolFibRetracement                 // fibonacci retracement levels
	DrawingToolTextLabel                      // freeform text annotation
	DrawingToolMeasure                        // temporary measure overlay (not persisted)
)

// DrawingID uniquely identifies a drawing within a chart.
type DrawingID string

// DataPoint holds a data-space coordinate pair.
type DataPoint struct {
	X, Y float64
}

// Drawing is the interface every persistent chart drawing implements.
// The drawingSealed method restricts implementations to this package.
type Drawing interface {
	ID() DrawingID
	Type() DrawingToolMode
	DrawingColor() color.Color
	SetDrawingColor(color.Color)
	DrawingLabel() string
	SetDrawingLabel(string)
	// DataPoints returns the defining data-space coordinates.
	DataPoints() []DataPoint
	// SetDataPoints replaces the defining coordinates (for drag-move/resize).
	SetDataPoints([]DataPoint)
	drawingSealed()
}

// DrawingBase provides shared fields for all drawing types.
type DrawingBase struct {
	ID_    DrawingID
	Color_ color.Color
	Label_ string
}

func (b *DrawingBase) ID() DrawingID                 { return b.ID_ }
func (b *DrawingBase) DrawingColor() color.Color     { return b.Color_ }
func (b *DrawingBase) SetDrawingColor(c color.Color) { b.Color_ = c }
func (b *DrawingBase) DrawingLabel() string          { return b.Label_ }
func (b *DrawingBase) SetDrawingLabel(l string)      { b.Label_ = l }
func (b *DrawingBase) drawingSealed()                {}

// --- Concrete drawing types ---

// HLineDrawing is a horizontal price level line spanning the full chart width.
type HLineDrawing struct {
	DrawingBase
	Y float64
}

func (d *HLineDrawing) Type() DrawingToolMode   { return DrawingToolHLine }
func (d *HLineDrawing) DataPoints() []DataPoint { return []DataPoint{{Y: d.Y}} }
func (d *HLineDrawing) SetDataPoints(pts []DataPoint) {
	if len(pts) > 0 {
		d.Y = pts[0].Y
	}
}

// VLineDrawing is a vertical time marker spanning the full chart height.
type VLineDrawing struct {
	DrawingBase
	X float64
}

func (d *VLineDrawing) Type() DrawingToolMode   { return DrawingToolVLine }
func (d *VLineDrawing) DataPoints() []DataPoint { return []DataPoint{{X: d.X}} }
func (d *VLineDrawing) SetDataPoints(pts []DataPoint) {
	if len(pts) > 0 {
		d.X = pts[0].X
	}
}

// TrendLineDrawing is a line segment between two data points.
type TrendLineDrawing struct {
	DrawingBase
	Start, End DataPoint
}

func (d *TrendLineDrawing) Type() DrawingToolMode   { return DrawingToolTrendLine }
func (d *TrendLineDrawing) DataPoints() []DataPoint { return []DataPoint{d.Start, d.End} }
func (d *TrendLineDrawing) SetDataPoints(pts []DataPoint) {
	if len(pts) >= 2 {
		d.Start, d.End = pts[0], pts[1]
	}
}

// RayDrawing is a line from Start extending infinitely through Direction.
type RayDrawing struct {
	DrawingBase
	Start, Direction DataPoint
}

func (d *RayDrawing) Type() DrawingToolMode   { return DrawingToolRay }
func (d *RayDrawing) DataPoints() []DataPoint { return []DataPoint{d.Start, d.Direction} }
func (d *RayDrawing) SetDataPoints(pts []DataPoint) {
	if len(pts) >= 2 {
		d.Start, d.Direction = pts[0], pts[1]
	}
}

// RectangleDrawing is a rectangular zone between two corners.
type RectangleDrawing struct {
	DrawingBase
	Corner1, Corner2 DataPoint
}

func (d *RectangleDrawing) Type() DrawingToolMode   { return DrawingToolRectangle }
func (d *RectangleDrawing) DataPoints() []DataPoint { return []DataPoint{d.Corner1, d.Corner2} }
func (d *RectangleDrawing) SetDataPoints(pts []DataPoint) {
	if len(pts) >= 2 {
		d.Corner1, d.Corner2 = pts[0], pts[1]
	}
}

// ChannelDrawing is two parallel trend lines.
// Line1Start→Line1End defines the primary line; OffsetY is the data-space
// vertical offset for the parallel line.
type ChannelDrawing struct {
	DrawingBase
	Line1Start, Line1End DataPoint
	OffsetY              float64
}

func (d *ChannelDrawing) Type() DrawingToolMode { return DrawingToolChannel }
func (d *ChannelDrawing) DataPoints() []DataPoint {
	return []DataPoint{d.Line1Start, d.Line1End, {Y: d.OffsetY}}
}

func (d *ChannelDrawing) SetDataPoints(pts []DataPoint) {
	if len(pts) >= 3 {
		d.Line1Start, d.Line1End = pts[0], pts[1]
		d.OffsetY = pts[2].Y
	}
}

// FibRetracementDrawing draws horizontal lines at fibonacci levels between HighY and LowY.
// Levels: 0%, 23.6%, 38.2%, 50%, 61.8%, 78.6%, 100%.
type FibRetracementDrawing struct {
	DrawingBase
	HighY, LowY float64
	X           float64 // anchor X for label positioning
}

// FibLevels are the standard fibonacci retracement percentages.
var FibLevels = []float64{0, 0.236, 0.382, 0.5, 0.618, 0.786, 1.0}

func (d *FibRetracementDrawing) Type() DrawingToolMode { return DrawingToolFibRetracement }
func (d *FibRetracementDrawing) DataPoints() []DataPoint {
	return []DataPoint{{X: d.X, Y: d.HighY}, {X: d.X, Y: d.LowY}}
}

func (d *FibRetracementDrawing) SetDataPoints(pts []DataPoint) {
	if len(pts) >= 2 {
		d.HighY = pts[0].Y
		d.LowY = pts[1].Y
		d.X = pts[0].X
	}
}

// TextLabelDrawing is a freeform text annotation at a data point.
type TextLabelDrawing struct {
	DrawingBase
	Pos  DataPoint
	Text string
}

func (d *TextLabelDrawing) Type() DrawingToolMode   { return DrawingToolTextLabel }
func (d *TextLabelDrawing) DataPoints() []DataPoint { return []DataPoint{d.Pos} }
func (d *TextLabelDrawing) SetDataPoints(pts []DataPoint) {
	if len(pts) > 0 {
		d.Pos = pts[0]
	}
}

// --- DrawingStore ---

// DrawingStore manages an ordered collection of drawings.
type DrawingStore struct {
	drawings []Drawing
	nextSeq  int
}

// NewDrawingStore creates an empty DrawingStore.
func NewDrawingStore() *DrawingStore {
	return &DrawingStore{}
}

// NextID generates a DrawingID that no drawing in the store uses, skipping
// any an app already gave its own drawings (#343).
func (ds *DrawingStore) NextID() DrawingID {
	for {
		ds.nextSeq++
		id := DrawingID(fmt.Sprintf("drawing-%d", ds.nextSeq))
		if ds.Get(id) == nil {
			return id
		}
	}
}

// Add appends a drawing.
func (ds *DrawingStore) Add(d Drawing) {
	ds.drawings = append(ds.drawings, d)
}

// Remove deletes a drawing by ID. Returns true if found.
func (ds *DrawingStore) Remove(id DrawingID) bool {
	for i, d := range ds.drawings {
		if d.ID() == id {
			ds.drawings = append(ds.drawings[:i], ds.drawings[i+1:]...)
			return true
		}
	}
	return false
}

// Get returns a drawing by ID, or nil if not found.
func (ds *DrawingStore) Get(id DrawingID) Drawing {
	for _, d := range ds.drawings {
		if d.ID() == id {
			return d
		}
	}
	return nil
}

// All returns all drawings in insertion order.
func (ds *DrawingStore) All() []Drawing {
	return ds.drawings
}

// Clear removes all drawings.
func (ds *DrawingStore) Clear() {
	ds.drawings = nil
}

// Len returns the number of drawings.
func (ds *DrawingStore) Len() int {
	return len(ds.drawings)
}

// --- DrawingGestureState ---

// DrawingGestureState tracks an in-progress drawing gesture.
type DrawingGestureState struct {
	Active  bool
	Tool    DrawingToolMode
	Step    int          // 0=waiting for first click, 1=have first point, etc.
	Points  [3]DataPoint // up to 3 data-space points
	Color   color.Color  // pre-assigned color for this drawing
	TextBuf string       // accumulated text for TextLabel tool

	// Measure tool: pixel-space tracking (temporary, no persistence).
	MeasureStart  geo.Position
	MeasureEnd    geo.Position
	MeasureActive bool

	// Drag-to-move: tracks moving a selected drawing.
	DragDrawingID DrawingID   // drawing being dragged ("" = none)
	DragOrigin    DataPoint   // data-space position where drag started
	DragOrigPts   []DataPoint // original data points before drag started
}

// Reset clears the gesture state.
func (g *DrawingGestureState) Reset() {
	*g = DrawingGestureState{}
}

// --- Drawing hit testing ---

// DrawingHitResult describes a hit on an existing drawing.
type DrawingHitResult struct {
	DrawingID DrawingID
	HandleIdx int // -1 = body hit, 0+ = specific drag handle
	Distance  float32
}

// HitTestDrawings checks if a pixel position hits any drawing.
// Returns the nearest hit within threshold, or nil.
func HitTestDrawings(
	drawings []Drawing,
	pos geo.Position,
	coordSystems []coord.System,
	plotRect coord.Rect,
	threshold float32,
) *DrawingHitResult {
	if len(coordSystems) == 0 {
		return nil
	}
	cs := coordSystems[0]

	var best *DrawingHitResult
	bestDist := threshold + 1

	for _, d := range drawings {
		dist := drawingDistance(d, pos, cs, plotRect)
		if dist < bestDist {
			bestDist = dist
			best = &DrawingHitResult{
				DrawingID: d.ID(),
				HandleIdx: -1,
				Distance:  dist,
			}
		}
	}

	if best != nil && bestDist <= threshold {
		return best
	}
	return nil
}

// drawingDistance returns the pixel distance from pos to the nearest part of a drawing.
func drawingDistance(d Drawing, pos geo.Position, cs coord.System, plotRect coord.Rect) float32 {
	switch dt := d.(type) {
	case *HLineDrawing:
		py := cs.DataToPixel(0, dt.Y).Y
		return abs32(pos.Y - py)
	case *VLineDrawing:
		px := cs.DataToPixel(dt.X, 0).X
		return abs32(pos.X - px)
	case *TrendLineDrawing:
		p1 := cs.DataToPixel(dt.Start.X, dt.Start.Y)
		p2 := cs.DataToPixel(dt.End.X, dt.End.Y)
		return pointToSegmentDist(pos, p1, p2)
	case *RayDrawing:
		p1 := cs.DataToPixel(dt.Start.X, dt.Start.Y)
		p2 := cs.DataToPixel(dt.Direction.X, dt.Direction.Y)
		return pointToRayDist(pos, p1, p2)
	case *RectangleDrawing:
		p1 := cs.DataToPixel(dt.Corner1.X, dt.Corner1.Y)
		p2 := cs.DataToPixel(dt.Corner2.X, dt.Corner2.Y)
		return pointToRectEdgeDist(pos, p1, p2)
	case *ChannelDrawing:
		p1 := cs.DataToPixel(dt.Line1Start.X, dt.Line1Start.Y)
		p2 := cs.DataToPixel(dt.Line1End.X, dt.Line1End.Y)
		d1 := pointToSegmentDist(pos, p1, p2)
		p3 := cs.DataToPixel(dt.Line1Start.X, dt.Line1Start.Y+dt.OffsetY)
		p4 := cs.DataToPixel(dt.Line1End.X, dt.Line1End.Y+dt.OffsetY)
		d2 := pointToSegmentDist(pos, p3, p4)
		if d1 < d2 {
			return d1
		}
		return d2
	case *FibRetracementDrawing:
		span := dt.HighY - dt.LowY
		minDist := float32(math.MaxFloat32)
		for _, level := range FibLevels {
			y := dt.LowY + span*level
			py := cs.DataToPixel(0, y).Y
			d := abs32(pos.Y - py)
			if d < minDist {
				minDist = d
			}
		}
		return minDist
	case *TextLabelDrawing:
		// Text renders above the anchor point. Use bounding box hit test.
		p := cs.DataToPixel(dt.Pos.X, dt.Pos.Y)
		text := dt.Text
		if text == "" {
			text = "Label"
		}
		labelW := float32(len(text))*7 + 8
		labelH := float32(18)
		// Bounding box: (p.X, p.Y-labelH) to (p.X+labelW, p.Y)
		if pos.X >= p.X && pos.X <= p.X+labelW && pos.Y >= p.Y-labelH && pos.Y <= p.Y {
			return 0 // inside the label box
		}
		// Distance to nearest edge of the bounding box
		return pointToRectEdgeDist(pos, geo.NewPos(p.X, p.Y-labelH), geo.NewPos(p.X+labelW, p.Y))
	}
	return float32(math.MaxFloat32)
}

func abs32(f float32) float32 {
	if f < 0 {
		return -f
	}
	return f
}

// pointToSegmentDist returns the distance from point p to line segment (a, b).
func pointToSegmentDist(p, a, b geo.Position) float32 {
	dx := b.X - a.X
	dy := b.Y - a.Y
	lenSq := dx*dx + dy*dy
	if lenSq == 0 {
		ddx := p.X - a.X
		ddy := p.Y - a.Y
		return float32(math.Sqrt(float64(ddx*ddx + ddy*ddy)))
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / lenSq
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	projX := a.X + t*dx
	projY := a.Y + t*dy
	ddx := p.X - projX
	ddy := p.Y - projY
	return float32(math.Sqrt(float64(ddx*ddx + ddy*ddy)))
}

// pointToRayDist returns the distance from point p to a ray starting at a going through b.
func pointToRayDist(p, a, b geo.Position) float32 {
	dx := b.X - a.X
	dy := b.Y - a.Y
	lenSq := dx*dx + dy*dy
	if lenSq == 0 {
		ddx := p.X - a.X
		ddy := p.Y - a.Y
		return float32(math.Sqrt(float64(ddx*ddx + ddy*ddy)))
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / lenSq
	if t < 0 {
		t = 0
	}
	// No upper clamp — ray extends infinitely.
	projX := a.X + t*dx
	projY := a.Y + t*dy
	ddx := p.X - projX
	ddy := p.Y - projY
	return float32(math.Sqrt(float64(ddx*ddx + ddy*ddy)))
}

// pointToRectEdgeDist returns the distance from p to the nearest edge of the rectangle
// defined by two opposite corners.
func pointToRectEdgeDist(p, c1, c2 geo.Position) float32 {
	minX, maxX := c1.X, c2.X
	if minX > maxX {
		minX, maxX = maxX, minX
	}
	minY, maxY := c1.Y, c2.Y
	if minY > maxY {
		minY, maxY = maxY, minY
	}

	// If inside, return distance to nearest edge.
	if p.X >= minX && p.X <= maxX && p.Y >= minY && p.Y <= maxY {
		dists := [4]float32{p.X - minX, maxX - p.X, p.Y - minY, maxY - p.Y}
		best := dists[0]
		for _, d := range dists[1:] {
			if d < best {
				best = d
			}
		}
		return best
	}

	// Distance to the four edges.
	edges := [4][2]geo.Position{
		{geo.NewPos(minX, minY), geo.NewPos(maxX, minY)}, // top
		{geo.NewPos(maxX, minY), geo.NewPos(maxX, maxY)}, // right
		{geo.NewPos(minX, maxY), geo.NewPos(maxX, maxY)}, // bottom
		{geo.NewPos(minX, minY), geo.NewPos(minX, maxY)}, // left
	}
	best := float32(math.MaxFloat32)
	for _, edge := range edges {
		d := pointToSegmentDist(p, edge[0], edge[1])
		if d < best {
			best = d
		}
	}
	return best
}
