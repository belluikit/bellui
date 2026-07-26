package tables

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// collectHeaderLabels walks obj (recursing into the main-band clip/container),
// recording each bold header label's ABSOLUTE X — the clip-local label position
// folded together with its ancestors' offsets.
func collectHeaderLabels(obj core.CanvasObject, offsetX float32, m map[string]float32) {
	switch o := obj.(type) {
	case *container.Clip:
		collectHeaderLabels(o.Content, offsetX+o.Position().X, m)
	case *bellui.Container:
		base := offsetX + o.Position().X
		for _, child := range o.Objects {
			collectHeaderLabels(child, base, m)
		}
	case *canvas.Text:
		if o.TextStyle.Bold && o.Text != "" {
			m[o.Text] = offsetX + o.Position().X
		}
	}
}

// headerLabelX returns the absolute drawn X position of each header label
// (bold canvas texts), keyed by header text, from the live header renderer.
func headerLabelX(t *testing.T, g *DataGrid) map[string]float32 {
	t.Helper()
	r := test.WidgetRenderer(g.headerWidget)
	m := make(map[string]float32)
	for _, o := range r.Objects() {
		collectHeaderLabels(o, 0, m)
	}
	return m
}

// The header shares the body's region layout: under horizontal scroll the main
// header labels translate by -offset.X while the pinned-left/right labels stay
// fixed (#58). This verifies the rendering (label geometry), complementing the
// hit-test round-trip tests.
func TestHeaderRender_MainScrollsPinnedFixed(t *testing.T) {
	data := make([]map[string]any, 10)
	for i := range data {
		data[i] = map[string]any{"a": i, "b": i, "c": i, "d": i}
	}
	// Header text defaults to the field name when HeaderName is unset.
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a", Pinned: PinLeft, Width: 80},
		{Field: "b", Width: 80},
		{Field: "c", Width: 80},
		{Field: "d", Pinned: PinRight, Width: 80},
	})
	w := test.NewWindow(g)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(600, 300)) // wide enough that every column fits

	before := headerLabelX(t, g)
	for _, k := range []string{"a", "b", "c", "d"} {
		if _, ok := before[k]; !ok {
			t.Fatalf("header label %q not rendered; got %v", k, before)
		}
	}

	// Drive the real scroll path (onScrolled must refresh the header).
	const scroll = 50
	g.onScrolled(geo.NewPos(scroll, 0))
	after := headerLabelX(t, g)

	// Pinned bands are fixed — unaffected by the main scroll.
	if before["a"] != after["a"] {
		t.Errorf("pinned-left label moved: %v -> %v (want fixed)", before["a"], after["a"])
	}
	if before["d"] != after["d"] {
		t.Errorf("pinned-right label moved: %v -> %v (want fixed)", before["d"], after["d"])
	}

	// Main labels translate left by exactly the scroll offset.
	for _, k := range []string{"b", "c"} {
		if got := before[k] - after[k]; got != scroll {
			t.Errorf("main label %q shifted by %v, want %v", k, got, float32(scroll))
		}
	}
}
