package interact

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/series"
)

func makeTestCoord() *coord.Cartesian2D {
	xScale := axis.NewLinearScale(5, nil, nil)
	yScale := axis.NewLinearScale(5, nil, nil)
	xScale.SetRange(0, 4)
	yScale.SetRange(0, 100)
	c := coord.NewCartesian2D(xScale, yScale)
	c.SetBounds(coord.Rect{X: 0, Y: 0, W: 400, H: 300})
	return c
}

func TestHitTester_FindNearest_Line(t *testing.T) {
	ht := NewHitTester()
	cs := makeTestCoord()
	data := []float64{10, 50, 90, 30, 70}
	ht.AddLineSeries(0, "test", series.TypeLine, cs, data)

	// Click near the pixel position of data[1] (value 50)
	p1 := cs.DataToPixel(1, 50)
	hit := ht.FindNearest(geo.NewPos(p1.X+2, p1.Y+2), 20)
	if hit == nil {
		t.Fatal("expected a hit")
	}
	if hit.DataIndex != 1 {
		t.Errorf("expected DataIndex 1, got %d", hit.DataIndex)
	}
	if hit.SeriesName != "test" {
		t.Errorf("expected SeriesName 'test', got %q", hit.SeriesName)
	}
}

func TestHitTester_FindNearest_Miss(t *testing.T) {
	ht := NewHitTester()
	cs := makeTestCoord()
	ht.AddLineSeries(0, "test", series.TypeLine, cs, []float64{10, 50})

	// Click far from any point
	hit := ht.FindNearest(geo.NewPos(9999, 9999), 20)
	if hit != nil {
		t.Error("expected no hit for distant click")
	}
}

func TestHitTester_FindAllAtX(t *testing.T) {
	ht := NewHitTester()
	cs := makeTestCoord()
	ht.AddLineSeries(0, "A", series.TypeLine, cs, []float64{10, 50, 90})
	ht.AddLineSeries(1, "B", series.TypeLine, cs, []float64{20, 60, 80})

	// Click near X position of data[1]
	p1 := cs.DataToPixel(1, 50)
	hits := ht.FindAllAtX(geo.NewPos(p1.X, 150), 30)
	if len(hits) != 2 {
		t.Errorf("expected 2 hits (axis trigger), got %d", len(hits))
	}
}

func TestHitTester_BarRect(t *testing.T) {
	ht := NewHitTester()
	cs := makeTestCoord()
	data := []float64{50, 80}
	ht.AddBarSeries(0, "bars", cs, data, 40) // barWidth=40px

	// Click inside the first bar
	p0 := cs.DataToPixel(0, 50)
	baseline := cs.DataToPixel(0, 0)
	midY := (p0.Y + baseline.Y) / 2
	hit := ht.FindNearest(geo.NewPos(p0.X, midY), 100)
	if hit == nil {
		t.Fatal("expected hit inside bar")
	}
	if hit.DataIndex != 0 {
		t.Errorf("expected DataIndex 0, got %d", hit.DataIndex)
	}
}

func TestHitTester_Pie(t *testing.T) {
	ht := NewHitTester()
	center := geo.NewPos(200, 200)
	radius := float32(100)
	sectors := []PieSectorData{
		MakePieSector(0, 0, 180, "Top", 50),
		MakePieSector(1, 180, 360, "Bottom", 50),
	}
	ht.AddPieSeries(0, "pie", center, radius, sectors)

	// Click at top (angle ~0, inside radius)
	hit := ht.FindNearest(geo.NewPos(200, 110), 200)
	if hit == nil {
		t.Fatal("expected hit in pie")
	}
	if hit.DataIndex != 0 {
		t.Errorf("expected top sector (index 0), got %d", hit.DataIndex)
	}

	// Click outside radius
	hit = ht.FindNearest(geo.NewPos(200, 50), 200)
	if hit != nil {
		t.Error("expected no hit outside pie radius")
	}
}

func TestHitTester_Clear(t *testing.T) {
	ht := NewHitTester()
	cs := makeTestCoord()
	ht.AddLineSeries(0, "test", series.TypeLine, cs, []float64{10})
	ht.Clear()

	p := cs.DataToPixel(0, 10)
	hit := ht.FindNearest(geo.NewPos(p.X, p.Y), 20)
	if hit != nil {
		t.Error("expected no hit after Clear")
	}
}
