package tables

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/tables/render"
)

// The column-header row publishes one accessible heading node per visible
// column (pinned + main), so screen-reader users can read the column titles —
// it used to expose no nodes at all (#109). Sortable headers carry a "press"
// action that cycles the column's sort through the normal header-tap path.
func TestDataGrid_HeaderCellsAccessible(t *testing.T) {
	data := []map[string]any{
		{"id": 2, "name": "Bob"},
		{"id": 1, "name": "Alice"},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "id", HeaderName: "ID", Width: 60, Pinned: PinLeft},
		{Field: "name", HeaderName: "Name"},
	})

	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(500, 300))

	// Collect the header's accessible heading nodes from the rendered tree.
	headings := map[string]core.Accessible{}
	var walk func(o core.CanvasObject)
	walk = func(o core.CanvasObject) {
		if a, ok := o.(core.Accessible); ok && a.AccessibleRole() == core.RoleHeading {
			headings[a.AccessibleLabel()] = a
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

	for _, want := range []string{"ID", "Name"} {
		if headings[want] == nil {
			t.Errorf("no accessible heading node for column %q (have %v)", want, headings)
		}
	}

	// Performing the Name header's press action sorts the column ascending,
	// exactly like a pointer tap on the header.
	nameNode, ok := headings["Name"].(core.AccessibleActioner)
	if !ok {
		t.Fatal("Name header node declares no actions")
	}
	acts := nameNode.AccessibleActions()
	if len(acts) != 1 || acts[0].Name != core.ActionPress {
		t.Fatalf("Name header actions = %v, want single press", acts)
	}
	nameNode.PerformAccessibleAction(core.ActionPress)

	// Absolute display column 1 is "name" (after the pinned ID column).
	if got := g.SortState(1); got != render.SortIndicatorAsc {
		t.Errorf("after press: SortState(name) = %v, want ascending", got)
	}
	if v, _ := g.pipe.DisplayRow(0).(map[string]any); v["name"] != "Alice" {
		t.Errorf("after press: first display row = %v, want Alice first", v)
	}
}
