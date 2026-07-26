package series

import (
	"math"
	"testing"
)

func TestRendererRegistry(t *testing.T) {
	// All registered types should return a non-nil renderer
	registeredTypes := []Type{
		TypeLine, TypeBar, TypeScatter, TypePie, TypeCandlestick,
		TypeHeatmap, TypeRadar, TypeGauge, TypeFunnel, TypeBoxplot,
		TypeTreemap, TypeSankey, TypeSunburst, TypeGraph, TypeTree,
		TypeParallel, TypePictorial, TypeEffectScatter, TypeCustom,
	}
	for _, st := range registeredTypes {
		r := NewRenderer(st)
		if r == nil {
			t.Errorf("NewRenderer(%q) returned nil, expected registered renderer", st)
		}
	}

	// Unregistered type should return nil
	if r := NewRenderer(Type("nonexistent")); r != nil {
		t.Error("expected nil for unregistered type")
	}
}

func TestLineSeries_YAxisIndexer(t *testing.T) {
	ls := &LineSeries{YAxisIndex: 2}
	var yi YAxisIndexer = ls
	if yi.GetYAxisIndex() != 2 {
		t.Errorf("GetYAxisIndex() = %d, want 2", yi.GetYAxisIndex())
	}
}

func TestBarSeries_Stackable(t *testing.T) {
	bs := &BarSeries{Stack: "group1", Data: []float64{1, 2, 3}}
	var st Stackable = bs
	if st.StackGroup() != "group1" {
		t.Errorf("StackGroup() = %q, want group1", st.StackGroup())
	}
	if len(st.StackData()) != 3 {
		t.Errorf("StackData() len = %d, want 3", len(st.StackData()))
	}
}

func TestLineSeries_DataRange(t *testing.T) {
	ls := &LineSeries{Data: []float64{10, 50, math.NaN(), 30, 70}}
	var dr DataRanger = ls
	xMin, xMax, yMin, yMax := dr.DataRange()

	if xMin != 0 || xMax != 4 {
		t.Errorf("X range = [%v, %v], want [0, 4]", xMin, xMax)
	}
	if yMin != 10 || yMax != 70 {
		t.Errorf("Y range = [%v, %v], want [10, 70]", yMin, yMax)
	}
}

func TestBarSeries_DataRange_IncludesZero(t *testing.T) {
	bs := &BarSeries{Data: []float64{5, 10, 15}}
	xMin, xMax, yMin, yMax := bs.DataRange()

	if yMin != 0 {
		t.Errorf("Bar Y min = %v, want 0 (bars always include baseline)", yMin)
	}
	if xMin != 0 || xMax != 2 {
		t.Errorf("X range = [%v, %v], want [0, 2]", xMin, xMax)
	}
	if yMax != 15 {
		t.Errorf("Y max = %v, want 15", yMax)
	}
}

func TestScatterSeries_DataRange(t *testing.T) {
	ss := &ScatterSeries{Data: []float64{1, 10, 3, 30, 2, 20}}
	xMin, xMax, yMin, yMax := ss.DataRange()

	if xMin != 1 || xMax != 3 {
		t.Errorf("X range = [%v, %v], want [1, 3]", xMin, xMax)
	}
	if yMin != 10 || yMax != 30 {
		t.Errorf("Y range = [%v, %v], want [10, 30]", yMin, yMax)
	}
}

func TestCandlestickSeries_DataRange(t *testing.T) {
	cs := &CandlestickSeries{Data: []CandlestickData{
		{Open: 10, High: 15, Low: 5, Close: 12},
		{Open: 12, High: 20, Low: 8, Close: 18},
	}}
	xMin, xMax, yMin, yMax := cs.DataRange()

	if xMin != 0 || xMax != 1 {
		t.Errorf("X range = [%v, %v], want [0, 1]", xMin, xMax)
	}
	if yMin != 5 || yMax != 20 {
		t.Errorf("Y range = [%v, %v], want [5, 20]", yMin, yMax)
	}
}

func TestBoxplotSeries_DataRange(t *testing.T) {
	bs := &BoxplotSeries{Data: []BoxplotData{
		{Min: 1, Q1: 5, Median: 10, Q3: 15, Max: 20},
		{Min: 3, Q1: 7, Median: 12, Q3: 17, Max: 25},
	}}
	_, _, yMin, yMax := bs.DataRange()

	if yMin != 1 || yMax != 25 {
		t.Errorf("Y range = [%v, %v], want [1, 25]", yMin, yMax)
	}
}

func TestEmptySeriesDataRange(t *testing.T) {
	ls := &LineSeries{}
	xMin, xMax, yMin, yMax := ls.DataRange()
	if xMin != 0 || xMax != 0 || yMin != 0 || yMax != 0 {
		t.Errorf("empty series DataRange = [%v,%v,%v,%v], want all zeros", xMin, xMax, yMin, yMax)
	}
}

func TestDefaultConstants(t *testing.T) {
	// Sanity checks: all defaults should be positive
	defaults := []struct {
		name  string
		value float32
	}{
		{"DefaultBarWidth", DefaultBarWidth},
		{"DefaultBoxWidth", DefaultBoxWidth},
		{"DefaultLineWidth", DefaultLineWidth},
		{"DefaultLineSymbolSize", DefaultLineSymbolSize},
		{"DefaultScatterSize", DefaultScatterSize},
		{"DefaultPieRadius", DefaultPieRadius},
		{"DefaultGaugeRadius", DefaultGaugeRadius},
		{"DefaultAreaOpacity", DefaultAreaOpacity},
		{"DefaultRippleScale", DefaultRippleScale},
		{"DefaultPictorialSize", DefaultPictorialSize},
		{"DefaultGraphNodeSize", DefaultGraphNodeSize},
		{"DefaultTreeNodeSize", DefaultTreeNodeSize},
		{"DefaultParallelWidth", DefaultParallelWidth},
		{"DefaultBar3DSize", DefaultBar3DSize},
		{"DefaultScatter3DSize", DefaultScatter3DSize},
	}
	for _, d := range defaults {
		if d.value <= 0 {
			t.Errorf("%s = %v, should be positive", d.name, d.value)
		}
	}

	if len(DefaultPalette) < 4 {
		t.Errorf("DefaultPalette has %d colors, expected at least 4", len(DefaultPalette))
	}
}
