package tables

import (
	"fmt"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/x/tables/render"
)

// overlayClip returns the clip the group-row overlay draws its rows inside.
func overlayClip(t *testing.T, g *DataGrid) *container.Clip {
	t.Helper()
	objs := test.WidgetRenderer(g.groupRowOverlay).Objects()
	if len(objs) != 1 {
		t.Fatalf("overlay has %d top-level objects, want its one clip", len(objs))
	}
	clip, ok := objs[0].(*container.Clip)
	if !ok {
		t.Fatalf("overlay object is %T, want *container.Clip", objs[0])
	}
	return clip
}

// The group-row overlay is drawn above the main scroll, so it must leave the
// strips where the scroll draws its bars uncovered; otherwise each group row
// paints over the bar and the bar shows in segments. Under the strip the main
// band paints the group row in the group colour, so the row stays one band.
func TestDataGrid_GroupRowsLeaveScrollBarsUncovered(t *testing.T) {
	var data []map[string]any
	for i := range 60 {
		data = append(data, map[string]any{"dept": fmt.Sprintf("D%d", i%6), "a": i, "b": i, "c": i})
	}
	cols := []*ColumnDef{
		{Field: "dept", Width: 200},
		{Field: "a", Width: 200},
		{Field: "b", Width: 200},
		{Field: "c", Width: 200},
	}
	g := newTestGrid(t, data, cols)
	g.API().SetRowGroupColumn("dept")

	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(500, 300))

	barW := g.Theme().Size(bellui.ThemeSizeName("scrollBar"))
	if barW <= 0 {
		t.Fatalf("theme scrollBar size = %v; the test needs a real bar width", barW)
	}
	over := g.groupRowOverlay.Size()
	clip := overlayClip(t, g)
	if want := geo.NewSize(over.Width-barW, over.Height-barW); clip.Size() != want {
		t.Errorf("both bars shown: overlay clip = %v, want %v (overlay %v less the %v bars)", clip.Size(), want, over, barW)
	}

	// Under the strip, the main band paints group rows in the group colour.
	groupBg := render.ResolveThemeColors().GroupRowBg
	contentW := g.cellsWidget.Size().Width
	found := false
	for _, o := range test.WidgetRenderer(g.cellsWidget).Objects() {
		r, ok := o.(*canvas.Rectangle)
		if !ok || r.FillColor != groupBg {
			continue
		}
		found = true
		if r.Size().Width < contentW {
			t.Errorf("group row background is %v wide, want the band's full %v", r.Size().Width, contentW)
		}
	}
	if !found {
		t.Error("main band painted no group row in the group colour")
	}

	// Content that fits shows no bars, and the overlay covers its full size.
	w.Resize(geo.NewSize(1200, 3000))
	over = g.groupRowOverlay.Size()
	if clip := overlayClip(t, g); clip.Size() != over {
		t.Errorf("no bars shown: overlay clip = %v, want the full %v", clip.Size(), over)
	}
}
