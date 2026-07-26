package builder

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/x/charts/series"
)

func TestLineChartBuilder(t *testing.T) {
	opt := LineChart().
		Title("Test").
		CategoryX("Mon", "Tue", "Wed").
		ValueY().
		AddLine("Sales", 10, 20, 30).Symbol(series.SymbolCircle).Done().
		AddLine("Cost", 5, 15, 25).AreaFill(0.3).Done().
		Animate().
		Build()

	if opt.Title == nil || opt.Title.Text != "Test" {
		t.Error("title not set")
	}
	if len(opt.XAxis) != 1 || len(opt.XAxis[0].Data) != 3 {
		t.Error("X axis not configured")
	}
	if len(opt.YAxis) != 1 {
		t.Error("Y axis not configured")
	}
	if len(opt.Series) != 2 {
		t.Errorf("expected 2 series, got %d", len(opt.Series))
	}
	if !opt.Animation {
		t.Error("animation not enabled")
	}

	ls := opt.Series[0].(*series.LineSeries)
	if ls.SeriesName != "Sales" {
		t.Errorf("series 0 name = %q, want Sales", ls.SeriesName)
	}
	if ls.Symbol != series.SymbolCircle {
		t.Error("symbol not set")
	}

	ls2 := opt.Series[1].(*series.LineSeries)
	if ls2.AreaStyle == nil || ls2.AreaStyle.Opacity != 0.3 {
		t.Error("area fill not set on series 1")
	}
}

func TestBarChartBuilder(t *testing.T) {
	opt := BarChart().
		Title("Bars").
		CategoryX("Q1", "Q2").
		ValueY().
		AddBar("Rev", 100, 200).CornerRadius(4).Done().
		Build()

	if len(opt.Series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(opt.Series))
	}
	bs := opt.Series[0].(*series.BarSeries)
	if bs.CornerRadius != 4 {
		t.Errorf("corner radius = %v, want 4", bs.CornerRadius)
	}
}

func TestPieChartBuilder(t *testing.T) {
	opt := PieChart().
		Title("Pie").
		AddPie("Share").
		Slice("A", 60).
		Slice("B", 40).
		Radius(0.9).
		Labels().
		Done().
		Build()

	if len(opt.Series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(opt.Series))
	}
	ps := opt.Series[0].(*series.PieSeries)
	if len(ps.Data) != 2 {
		t.Errorf("expected 2 slices, got %d", len(ps.Data))
	}
	if ps.Radius != 0.9 {
		t.Errorf("radius = %v, want 0.9", ps.Radius)
	}
	if !ps.ShowLabels {
		t.Error("labels not enabled")
	}
}

func TestScatterChartBuilder(t *testing.T) {
	opt := ScatterChart().
		Title("Scatter").
		AddScatter("Pts", 1, 2, 3, 4).Size(12).Symbol(series.SymbolRect).Done().
		Build()

	ss := opt.Series[0].(*series.ScatterSeries)
	if ss.SymbolSize != 12 {
		t.Errorf("symbol size = %v, want 12", ss.SymbolSize)
	}
	if ss.PointCount() != 2 {
		t.Errorf("point count = %d, want 2", ss.PointCount())
	}
}

func TestChartBuilder_Widget(t *testing.T) {
	// Just verify it doesn't panic
	w := LineChart().
		Title("Test").
		CategoryX("A").
		ValueY().
		AddLine("X", 1).Done().
		Widget()
	if w == nil {
		t.Error("Widget() returned nil")
	}
}

func TestChartBuilder_Zoom(t *testing.T) {
	opt := LineChart().Zoom().Build()
	if opt.DataZoom == nil {
		t.Error("DataZoom not set")
	}
	if !opt.DataZoom.Enabled {
		t.Error("DataZoom not enabled")
	}
}

func TestChartBuilder_Brush(t *testing.T) {
	opt := ScatterChart().Brush().Build()
	if opt.Brush == nil || !opt.Brush.Enabled {
		t.Error("Brush not enabled")
	}
}

func TestChartBuilder_VisualMap(t *testing.T) {
	blue := color.NRGBA{B: 255, A: 255}
	red := color.NRGBA{R: 255, A: 255}

	opt := Chart().
		Title("Heatmap").
		VisualMap(0, 100).
		Colors(blue, red).
		SymbolSize(4, 20).
		Dimension(1).
		ForSeries(0).
		Done().
		Build()

	if len(opt.VisualMap) != 1 {
		t.Fatalf("expected 1 VisualMap, got %d", len(opt.VisualMap))
	}
	vm := opt.VisualMap[0]
	if vm.Min != 0 || vm.Max != 100 {
		t.Errorf("Min/Max = (%v, %v), want (0, 100)", vm.Min, vm.Max)
	}
	if len(vm.InRange.Colors) != 2 {
		t.Errorf("Colors count = %d, want 2", len(vm.InRange.Colors))
	}
	if vm.InRange.SymbolSizeMin != 4 || vm.InRange.SymbolSizeMax != 20 {
		t.Errorf("SymbolSize = (%v, %v), want (4, 20)", vm.InRange.SymbolSizeMin, vm.InRange.SymbolSizeMax)
	}
	if vm.Dimension != 1 {
		t.Errorf("Dimension = %d, want 1", vm.Dimension)
	}
	if vm.SeriesIndex != 0 {
		t.Errorf("SeriesIndex = %d, want 0", vm.SeriesIndex)
	}
}

func TestChartBuilder_MultipleVisualMaps(t *testing.T) {
	opt := Chart().
		VisualMap(0, 50).Dimension(0).Done().
		VisualMap(10, 200).ForSeries(1).Done().
		Build()

	if len(opt.VisualMap) != 2 {
		t.Fatalf("expected 2 VisualMaps, got %d", len(opt.VisualMap))
	}
	if opt.VisualMap[0].Max != 50 {
		t.Errorf("VisualMap[0].Max = %v, want 50", opt.VisualMap[0].Max)
	}
	if opt.VisualMap[1].SeriesIndex != 1 {
		t.Errorf("VisualMap[1].SeriesIndex = %d, want 1", opt.VisualMap[1].SeriesIndex)
	}
}
