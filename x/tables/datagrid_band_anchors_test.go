package tables

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// The body's drawn positions (doLayout) must match the band anchors hit-testing
// and the header/filter rows use (hitRegions), since both now come from the one
// bandAnchors source. This guards against the two drifting apart (#69): the
// pinned-left band starts at 0, the main scroll at mainX, and the pinned-right
// overlay at pinnedRightX — including when a narrow width forces the clamp.
func TestBodyLayout_MatchesBandAnchors(t *testing.T) {
	data := make([]map[string]any, 10)
	for i := range data {
		data[i] = map[string]any{"a": i, "b": i, "c": i, "d": i, "e": i}
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a", Pinned: PinLeft, Width: 80},
		{Field: "b", Width: 80},
		{Field: "c", Width: 80},
		{Field: "d", Width: 80},
		{Field: "e", Pinned: PinRight, Width: 80},
	})
	w := test.NewWindow(g)
	t.Cleanup(w.Close)

	for _, tc := range []struct {
		name  string
		width float32
	}{
		{"wide (no clamp)", 600},
		{"narrow (pinned-right clamped)", 300},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w.Resize(geo.NewSize(tc.width, 300))
			g.Refresh() // force doLayout at this size

			mainX, pinnedRightX, pinnedRightW := g.hitRegions()
			if pinnedRightW <= 0 {
				t.Fatal("expected a pinned-right band")
			}

			if got := g.pinnedLeftCells.Position().X; got != 0 {
				t.Errorf("pinned-left cells X = %v, want 0", got)
			}
			if got := g.scrollContent.Scroll.Position().X; got != mainX {
				t.Errorf("main scroll X = %v, want mainX %v", got, mainX)
			}
			if got := g.pinnedRightCells.Position().X; got != pinnedRightX {
				t.Errorf("pinned-right cells X = %v, want pinnedRightX %v", got, pinnedRightX)
			}
		})
	}
}
