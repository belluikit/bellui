package coord

import (
	"math"
	"testing"

	"github.com/belluikit/bellui/x/charts/axis"
)

func TestCartesian2D_DataToPixel(t *testing.T) {
	xScale := axis.NewLinearScale(5, nil, nil)
	yScale := axis.NewLinearScale(5, nil, nil)
	xScale.SetRange(0, 100)
	yScale.SetRange(0, 100)

	c := NewCartesian2D(xScale, yScale)
	c.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})

	// Data (0, 0) -> bottom-left of plot area (Y flipped)
	pos := c.DataToPixel(0, 0)
	xMin, _ := xScale.Range()
	yMin, _ := yScale.Range()
	xNorm := xScale.DataToNormalized(xMin)
	yNorm := yScale.DataToNormalized(yMin)
	wantX := float32(xNorm) * 200
	wantY := float32(1-yNorm) * 100 // Y is flipped
	if math.Abs(float64(pos.X-wantX)) > 1 || math.Abs(float64(pos.Y-wantY)) > 1 {
		t.Errorf("DataToPixel(0,0) = (%v,%v), want near (%v,%v)", pos.X, pos.Y, wantX, wantY)
	}
}

func TestCartesian2D_RoundTrip(t *testing.T) {
	xScale := axis.NewLinearScale(5, nil, nil)
	yScale := axis.NewLinearScale(5, nil, nil)
	xScale.SetRange(0, 100)
	yScale.SetRange(0, 100)

	c := NewCartesian2D(xScale, yScale)
	c.SetBounds(Rect{X: 50, Y: 20, W: 400, H: 300})

	// Test several data points for round-trip consistency
	testPts := [][2]float64{{10, 20}, {50, 50}, {90, 80}}
	for _, pt := range testPts {
		pos := c.DataToPixel(pt[0], pt[1])
		backX, backY := c.PixelToData(pos)
		if math.Abs(backX-pt[0]) > 1 || math.Abs(backY-pt[1]) > 1 {
			t.Errorf("round-trip (%v,%v) -> pixel (%v,%v) -> (%v,%v)",
				pt[0], pt[1], pos.X, pos.Y, backX, backY)
		}
	}
}

func TestCartesian2D_Invert(t *testing.T) {
	xScale := axis.NewLinearScale(5, nil, nil)
	yScale := axis.NewLinearScale(5, nil, nil)
	xScale.SetRange(0, 100)
	yScale.SetRange(0, 100)

	c := NewCartesian2D(xScale, yScale)
	c.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})

	normal := c.DataToPixel(100, 0)

	c.SetInvert(true, false)
	inverted := c.DataToPixel(100, 0)

	// X-inverted: max data should map to left instead of right
	if inverted.X >= normal.X {
		t.Errorf("X invert: inverted.X=%v should be < normal.X=%v", inverted.X, normal.X)
	}
}

func TestCartesian2D_Bounds(t *testing.T) {
	xScale := axis.NewLinearScale(5, nil, nil)
	yScale := axis.NewLinearScale(5, nil, nil)

	c := NewCartesian2D(xScale, yScale)
	bounds := Rect{X: 10, Y: 20, W: 300, H: 200}
	c.SetBounds(bounds)

	got := c.Bounds()
	if got != bounds {
		t.Errorf("Bounds = %v, want %v", got, bounds)
	}
}

func TestNone_Identity(t *testing.T) {
	n := NewNone()
	n.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})

	pos := n.DataToPixel(42, 73)
	if pos.X != 42 || pos.Y != 73 {
		t.Errorf("None.DataToPixel(42,73) = (%v,%v), want (42,73)", pos.X, pos.Y)
	}

	x, y := n.PixelToData(pos)
	if x != 42 || y != 73 {
		t.Errorf("None.PixelToData round-trip failed: (%v,%v)", x, y)
	}

	if n.XScale() != nil || n.YScale() != nil {
		t.Error("None should have nil scales")
	}
}
