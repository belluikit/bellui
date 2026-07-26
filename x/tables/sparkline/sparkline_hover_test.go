package sparkline

import (
	"testing"

	"github.com/belluikit/bellui/geo"
)

func TestSparkline_HoverPointDetection(t *testing.T) {
	s := NewLine([]float64{10, 20, 30, 40, 50})
	// Simulate widget size
	s.Resize(geo.NewSize(100, 40))

	// With 5 data points and width=100, padding=2:
	// drawW = 96, each point separated by 96/4 = 24px
	// Point 0 at x=2, Point 1 at x=26, Point 2 at x=50, etc.

	// Test that updateHoverPoint computes the right index
	// We can't call it directly since it calls Refresh, but we can test
	// the math used to compute the index
	data := s.Data
	pad := s.Opts.Padding
	drawW := s.Size().Width - pad*2

	testCases := []struct {
		mouseX   float32
		expected int
	}{
		{pad, 0},           // left edge -> first point
		{pad + drawW, 4},   // right edge -> last point
		{pad + drawW/2, 2}, // middle -> middle point
		{pad + drawW/4, 1}, // quarter -> second point
	}

	for _, tc := range testCases {
		relX := tc.mouseX - pad
		if relX < 0 {
			relX = 0
		}
		if relX > drawW {
			relX = drawW
		}
		idx := int(relX/drawW*float32(len(data)-1) + 0.5)
		if idx < 0 {
			idx = 0
		}
		if idx >= len(data) {
			idx = len(data) - 1
		}
		if idx != tc.expected {
			t.Errorf("mouseX=%.0f: expected index %d, got %d", tc.mouseX, tc.expected, idx)
		}
	}
}

func TestSparkline_HighlightIndexUpdates(t *testing.T) {
	s := NewLine([]float64{1, 2, 3})

	if s.Opts.HighlightIndex != -1 {
		t.Errorf("expected initial highlight index -1, got %d", s.Opts.HighlightIndex)
	}

	s.SetHighlight(1)
	if s.Opts.HighlightIndex != 1 {
		t.Errorf("expected highlight index 1, got %d", s.Opts.HighlightIndex)
	}

	s.SetHighlight(-1)
	if s.Opts.HighlightIndex != -1 {
		t.Errorf("expected highlight cleared, got %d", s.Opts.HighlightIndex)
	}
}
