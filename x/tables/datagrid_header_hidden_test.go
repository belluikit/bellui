package tables

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// An unset HeaderName falls back to Field, and HideHeaderText does not change
// the column's name — only what the header paints (#918).
func TestColumnDef_HeaderFallsBackToField(t *testing.T) {
	cases := []struct {
		def  ColumnDef
		want string
	}{
		{ColumnDef{Field: "play"}, "play"},
		{ColumnDef{Field: "play", HeaderName: "Play"}, "Play"},
		{ColumnDef{Field: "play", HideHeaderText: true}, "play"},
		{ColumnDef{Field: "play", HeaderName: "Play", HideHeaderText: true}, "Play"},
	}
	for _, c := range cases {
		if got := c.def.Header(); got != c.want {
			t.Errorf("%+v: Header() = %q, want %q", c.def, got, c.want)
		}
	}
}

// A HideHeaderText column paints a blank header cell, but its accessible
// heading still carries the column's name and its resize floor no longer
// reserves room for the hidden text (#918).
func TestHeaderRender_HideHeaderText(t *testing.T) {
	data := []map[string]any{{"playbutton_column": "▶", "title": "Song"}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "playbutton_column", HideHeaderText: true, Width: 120},
		{Field: "title", HeaderName: "Title", Width: 120},
	})
	w := test.NewWindow(g)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(500, 300))

	labels := headerLabelX(t, g)
	if _, ok := labels["Title"]; !ok {
		t.Fatalf("visible header %q not painted; got %v", "Title", labels)
	}
	if _, ok := labels["playbutton_column"]; ok {
		t.Errorf("hidden header text painted; got %v", labels)
	}

	headings := map[string]bool{}
	var walk func(o core.CanvasObject)
	walk = func(o core.CanvasObject) {
		if a, ok := o.(core.Accessible); ok && a.AccessibleRole() == core.RoleHeading {
			headings[a.AccessibleLabel()] = true
		}
		switch v := o.(type) {
		case *bellui.Container:
			for _, c := range v.Objects {
				walk(c)
			}
		case *container.Clip:
			walk(v.Content)
		case core.Widget:
			for _, c := range test.WidgetRenderer(v).Objects() {
				walk(c)
			}
		}
	}
	walk(g.headerWidget)
	if !headings["playbutton_column"] {
		t.Errorf("hidden-text column has no accessible heading by its name; have %v", headings)
	}

	// Drag the column narrower than its (hidden) title: the floor is the
	// grid's 40px minimum, not the title's measured width.
	g.OnResizeDragStart(0, 120)
	g.OnResizeDrag(0)
	g.OnResizeDragEnd()
	if got := g.visibleCol(0).width; got != 40 {
		t.Errorf("width after shrinking a hidden-text column = %v, want 40", got)
	}
}
