package interact

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/coord"
)

func TestDrawingStore_AddRemoveGet(t *testing.T) {
	ds := NewDrawingStore()

	id := ds.NextID()
	d := &HLineDrawing{DrawingBase: DrawingBase{ID_: id, Color_: color.White}, Y: 100}
	ds.Add(d)

	if ds.Len() != 1 {
		t.Fatalf("expected 1 drawing, got %d", ds.Len())
	}
	if got := ds.Get(id); got != d {
		t.Errorf("Get(%s) returned wrong drawing", id)
	}
	if ds.Get("nonexistent") != nil {
		t.Error("Get for nonexistent ID should return nil")
	}

	if !ds.Remove(id) {
		t.Error("Remove should return true for existing drawing")
	}
	if ds.Len() != 0 {
		t.Error("expected 0 drawings after remove")
	}
	if ds.Remove(id) {
		t.Error("Remove should return false for already-removed drawing")
	}
}

func TestDrawingStore_NextID_Unique(t *testing.T) {
	ds := NewDrawingStore()
	ids := make(map[DrawingID]bool)
	for i := 0; i < 100; i++ {
		id := ds.NextID()
		if ids[id] {
			t.Fatalf("duplicate ID: %s", id)
		}
		ids[id] = true
	}
}

func TestDrawingStore_Clear(t *testing.T) {
	ds := NewDrawingStore()
	ds.Add(&HLineDrawing{DrawingBase: DrawingBase{ID_: ds.NextID()}, Y: 50})
	ds.Add(&VLineDrawing{DrawingBase: DrawingBase{ID_: ds.NextID()}, X: 3})
	ds.Clear()
	if ds.Len() != 0 {
		t.Errorf("expected 0 after Clear, got %d", ds.Len())
	}
}

func TestDrawingTypes_DataPoints(t *testing.T) {
	tests := []struct {
		name string
		d    Drawing
		nPts int
		tool DrawingToolMode
	}{
		{"HLine", &HLineDrawing{DrawingBase: DrawingBase{ID_: "h1"}, Y: 42}, 1, DrawingToolHLine},
		{"VLine", &VLineDrawing{DrawingBase: DrawingBase{ID_: "v1"}, X: 7}, 1, DrawingToolVLine},
		{"TrendLine", &TrendLineDrawing{DrawingBase: DrawingBase{ID_: "t1"}, Start: DataPoint{1, 2}, End: DataPoint{3, 4}}, 2, DrawingToolTrendLine},
		{"Ray", &RayDrawing{DrawingBase: DrawingBase{ID_: "r1"}, Start: DataPoint{0, 0}, Direction: DataPoint{1, 1}}, 2, DrawingToolRay},
		{"Rectangle", &RectangleDrawing{DrawingBase: DrawingBase{ID_: "rc1"}, Corner1: DataPoint{0, 0}, Corner2: DataPoint{5, 5}}, 2, DrawingToolRectangle},
		{"Channel", &ChannelDrawing{DrawingBase: DrawingBase{ID_: "ch1"}, Line1Start: DataPoint{0, 0}, Line1End: DataPoint{5, 5}, OffsetY: 2}, 3, DrawingToolChannel},
		{"Fib", &FibRetracementDrawing{DrawingBase: DrawingBase{ID_: "f1"}, HighY: 100, LowY: 50, X: 3}, 2, DrawingToolFibRetracement},
		{"TextLabel", &TextLabelDrawing{DrawingBase: DrawingBase{ID_: "tl1"}, Pos: DataPoint{2, 3}, Text: "hi"}, 1, DrawingToolTextLabel},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.d.Type() != tt.tool {
				t.Errorf("Type() = %v, want %v", tt.d.Type(), tt.tool)
			}
			pts := tt.d.DataPoints()
			if len(pts) != tt.nPts {
				t.Errorf("DataPoints() returned %d points, want %d", len(pts), tt.nPts)
			}
		})
	}
}

func TestDrawingGestureState_Reset(t *testing.T) {
	g := DrawingGestureState{Active: true, Tool: DrawingToolHLine, Step: 2}
	g.Reset()
	if g.Active || g.Tool != DrawingToolNone || g.Step != 0 {
		t.Error("Reset should zero all fields")
	}
}

func makeDrawingTestCoord() coord.System {
	xScale := axis.NewLinearScale(5, nil, nil)
	yScale := axis.NewLinearScale(5, nil, nil)
	xScale.SetRange(0, 10)
	yScale.SetRange(0, 100)
	c := coord.NewCartesian2D(xScale, yScale)
	c.SetBounds(coord.Rect{X: 0, Y: 0, W: 500, H: 300})
	return c
}

func TestHitTestDrawings_HLine(t *testing.T) {
	cs := makeDrawingTestCoord()
	drawings := []Drawing{
		&HLineDrawing{DrawingBase: DrawingBase{ID_: "h1"}, Y: 50},
	}
	// Y=50 should map to the middle of the plot (150px)
	midPx := cs.DataToPixel(0, 50)

	hit := HitTestDrawings(drawings, geo.NewPos(100, midPx.Y+2), []coord.System{cs}, coord.Rect{W: 500, H: 300}, 10)
	if hit == nil {
		t.Fatal("expected hit on HLine")
	}
	if hit.DrawingID != "h1" {
		t.Errorf("expected hit on h1, got %s", hit.DrawingID)
	}
}

func TestHitTestDrawings_Miss(t *testing.T) {
	cs := makeDrawingTestCoord()
	drawings := []Drawing{
		&HLineDrawing{DrawingBase: DrawingBase{ID_: "h1"}, Y: 50},
	}
	// Click far away from the line
	hit := HitTestDrawings(drawings, geo.NewPos(100, 10), []coord.System{cs}, coord.Rect{W: 500, H: 300}, 5)
	if hit != nil {
		t.Error("expected no hit far from line")
	}
}

func TestHitTestDrawings_TrendLine(t *testing.T) {
	cs := makeDrawingTestCoord()
	drawings := []Drawing{
		&TrendLineDrawing{
			DrawingBase: DrawingBase{ID_: "t1"},
			Start:       DataPoint{0, 0},
			End:         DataPoint{10, 100},
		},
	}
	// The line goes from (0,300) to (500,0) in pixels (diagonal).
	// Midpoint is (250, 150).
	hit := HitTestDrawings(drawings, geo.NewPos(250, 150), []coord.System{cs}, coord.Rect{W: 500, H: 300}, 10)
	if hit == nil {
		t.Fatal("expected hit on TrendLine at midpoint")
	}
	if hit.DrawingID != "t1" {
		t.Errorf("expected t1, got %s", hit.DrawingID)
	}
}
