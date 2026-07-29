package tables

import (
	"strings"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// walkCanvasObjects visits o and every canvas object beneath it, descending
// into containers, clips, and widget renderers.
func walkCanvasObjects(o core.CanvasObject, visit func(core.CanvasObject)) {
	visit(o)
	switch v := o.(type) {
	case *bellui.Container:
		for _, c := range v.Objects {
			walkCanvasObjects(c, visit)
		}
	case *container.Clip:
		walkCanvasObjects(v.Content, visit)
	case core.Widget:
		for _, c := range test.WidgetRenderer(v).Objects() {
			walkCanvasObjects(c, visit)
		}
	}
}

// With pinned columns active, a group-header row must draw its label exactly
// once (#110 — it used to render per horizontal band, colliding at the pinned
// edge) and its accessible RoleTableRow node must span the full grid width so
// the row is reachable over the pinned zone too (#111).
func TestDataGrid_GroupHeaderOnceAcrossBands(t *testing.T) {
	data := []map[string]any{
		{"id": 1, "dept": "A", "name": "Alice", "status": "ok"},
		{"id": 2, "dept": "B", "name": "Bob", "status": "ok"},
		{"id": 3, "dept": "A", "name": "Charlie", "status": "ok"},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "id", Width: 60, Pinned: PinLeft},
		{Field: "dept"},
		{Field: "name"},
		{Field: "status", Width: 80, Pinned: PinRight},
	})
	g.API().SetRowGroupColumn("dept")

	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(600, 400))

	labelCount := 0
	var rowNodes []core.Accessible
	walkCanvasObjects(g, func(o core.CanvasObject) {
		if txt, ok := o.(*canvas.Text); ok && strings.HasPrefix(txt.Text, "A (") {
			labelCount++
		}
		if a, ok := o.(core.Accessible); ok && a.AccessibleRole() == core.RoleTableRow {
			rowNodes = append(rowNodes, a)
		}
	})

	if labelCount != 1 {
		t.Errorf("group label for dept A drawn %d times, want exactly 1", labelCount)
	}
	if len(rowNodes) != 2 {
		t.Fatalf("found %d RoleTableRow nodes, want 2 (one per group)", len(rowNodes))
	}
	gridW := g.Size().Width
	for _, n := range rowNodes {
		obj := n.(core.CanvasObject)
		if obj.Position().X != 0 || obj.Size().Width != gridW {
			t.Errorf("group node %q bounds x=%v w=%v, want x=0 w=%v (spanning pinned bands)",
				n.AccessibleLabel(), obj.Position().X, obj.Size().Width, gridW)
		}
	}
}
