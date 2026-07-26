package sparkline

import (
	"image/color"
	"testing"
)

func TestNewLine(t *testing.T) {
	data := []float64{1, 3, 2, 5, 4}
	s := NewLine(data)
	if s.ChartType != Line {
		t.Errorf("expected Line type, got %d", s.ChartType)
	}
	if len(s.Data) != 5 {
		t.Errorf("expected 5 data points, got %d", len(s.Data))
	}
}

func TestNewArea(t *testing.T) {
	s := NewArea([]float64{1, 2, 3})
	if s.ChartType != Area {
		t.Errorf("expected Area type, got %d", s.ChartType)
	}
}

func TestNewBar(t *testing.T) {
	s := NewBar([]float64{1, 2, 3})
	if s.ChartType != Bar {
		t.Errorf("expected Bar type, got %d", s.ChartType)
	}
}

func TestNewColumn(t *testing.T) {
	s := NewColumn([]float64{1, 2, 3})
	if s.ChartType != Column {
		t.Errorf("expected Column type, got %d", s.ChartType)
	}
}

func TestSparkline_Options(t *testing.T) {
	s := NewLine([]float64{1, 2}, func(o *Options) {
		o.Stroke = color.NRGBA{R: 255, A: 255}
		o.StrokeWidth = 3
		o.ShowMarkers = true
		o.MarkerSize = 4
	})
	if s.Opts.StrokeWidth != 3 {
		t.Errorf("expected StrokeWidth 3, got %f", s.Opts.StrokeWidth)
	}
	if !s.Opts.ShowMarkers {
		t.Error("expected ShowMarkers true")
	}
}

func TestSparkline_SetData(t *testing.T) {
	s := NewLine([]float64{1, 2})
	s.SetData([]float64{3, 4, 5})
	if len(s.Data) != 3 {
		t.Errorf("expected 3 data points after SetData, got %d", len(s.Data))
	}
}

func TestSparklineRenderer_Generate(t *testing.T) {
	s := NewLine([]float64{1, 3, 2, 5, 4})
	r := newSparklineRenderer(s)
	img := r.generate(100, 50)
	bounds := img.Bounds()
	if bounds.Dx() != 100 || bounds.Dy() != 50 {
		t.Errorf("expected 100x50, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestSparklineRenderer_EmptyData(t *testing.T) {
	s := NewLine(nil)
	r := newSparklineRenderer(s)
	img := r.generate(100, 50)
	if img == nil {
		t.Error("expected non-nil image even with empty data")
	}
}

func TestSparklineRenderer_ColumnType(t *testing.T) {
	s := NewColumn([]float64{10, 20, 30, 40})
	r := newSparklineRenderer(s)
	img := r.generate(100, 50)
	bounds := img.Bounds()
	if bounds.Dx() != 100 {
		t.Errorf("expected width 100, got %d", bounds.Dx())
	}
}

func TestToNRGBA(t *testing.T) {
	c := color.NRGBA{R: 100, G: 200, B: 50, A: 255}
	result := toNRGBA(c)
	if result != c {
		t.Errorf("expected identity conversion, got %v", result)
	}

	result = toNRGBA(nil)
	if result != (color.NRGBA{}) {
		t.Errorf("expected zero for nil, got %v", result)
	}
}
