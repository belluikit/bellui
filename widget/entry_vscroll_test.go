package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

func TestMultiLineEntry_InsideVScroll(t *testing.T) {
	e := widget.NewMultiLineEntry()
	e.SetText("line1\nline2\nline3\nline4\nline5")
	e.SetMinRowsVisible(5)

	// Place inside a VScroll (like a notebook cell)
	vbox := container.NewVBox(e)
	scroll := container.NewVScroll(vbox)

	w := test.NewWindow(scroll)
	w.Resize(geo.NewSize(400, 300))

	// Check row metrics — they should be valid
	top, stride := e.RowMetrics()
	if stride <= 0 {
		t.Errorf("RowMetrics stride=%f inside VScroll, expected > 0", stride)
	}
	if top < 0 {
		t.Errorf("RowMetrics topOffset=%f, expected >= 0", top)
	}

	// Check that row positions are distinct (not all at Y=0)
	positions, _ := e.RowPositions()
	if len(positions) < 2 {
		t.Fatalf("Expected at least 2 row positions, got %d", len(positions))
	}
	if positions[0] == positions[1] {
		t.Errorf("Rows 0 and 1 have same Y position (%f) — rendering will overlap", positions[0])
	}
}
