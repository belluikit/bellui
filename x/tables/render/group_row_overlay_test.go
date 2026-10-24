package render

import (
	"strings"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

func newOverlayViewport(g *fakeGrid, w, h float32) *Viewport {
	vp := &Viewport{
		DefaultRowHeight: 30,
		Padding:          2,
		TotalRows:        g.rows,
		TotalCols:        g.cols,
		ColWidths:        g.ColumnWidths(),
	}
	vp.RecomputeColPositions()
	vp.Update(geo.NewPos(0, 0), geo.NewSize(w, h))
	return vp
}

// overlayContent returns the overlay renderer's group-row objects (the content
// inside its clip container).
func overlayContent(r *groupRowOverlayRenderer) []core.CanvasObject {
	if r.band == nil {
		return nil
	}
	return r.band.Objects
}

// The overlay draws each visible group row exactly once, and its accessible
// RoleTableRow node spans the overlay's full width — pinned-left band through
// pinned-right — so touch exploration finds the row over any band (#111).
func TestGroupRowOverlay_LabelOncePerRow_NodeSpansFullWidth(t *testing.T) {
	test.NewTempApp(t)
	g := &fakeGrid{
		rows: 20, cols: 2, pinnedLeft: 1, pinnedRt: 1,
		groupRows: map[int]fakeGroupInfo{
			0: {key: "West", count: 125, expanded: true},
			4: {key: "East", count: 42, expanded: false, depth: 1},
		},
	}
	vp := newOverlayViewport(g, 400, 300)

	w := NewGroupRowOverlay(g, vp)
	size := geo.NewSize(600, 300)
	r := w.CreateRenderer().(*groupRowOverlayRenderer)
	r.refresh(size)

	var labels []*canvas.Text
	var nodes []*groupRowA11y
	for _, o := range overlayContent(r) {
		switch v := o.(type) {
		case *canvas.Text:
			if strings.Contains(v.Text, "(") {
				labels = append(labels, v)
			}
		case *groupRowA11y:
			nodes = append(nodes, v)
		}
	}

	if len(labels) != 2 {
		t.Fatalf("drew %d group labels, want 2 (one per visible group row)", len(labels))
	}
	byText := map[string]bool{}
	for _, l := range labels {
		if byText[l.Text] {
			t.Errorf("label %q drawn more than once", l.Text)
		}
		byText[l.Text] = true
	}
	if !byText["West (125)"] || !byText["East (42)"] {
		t.Errorf("labels = %v, want West (125) and East (42)", byText)
	}

	if len(nodes) != 2 {
		t.Fatalf("realized %d a11y nodes, want 2", len(nodes))
	}
	for _, n := range nodes {
		if n.AccessibleRole() != core.RoleTableRow {
			t.Errorf("node %q role = %v, want RoleTableRow", n.label, n.AccessibleRole())
		}
		if n.Position().X != 0 || n.Size().Width != size.Width {
			t.Errorf("node %q bounds x=%v w=%v, want x=0 w=%v (full row incl. pinned bands)",
				n.label, n.Position().X, n.Size().Width, size.Width)
		}
	}

	// The expanded group reports StateExpanded; the collapsed one does not.
	byLabel := map[string]*groupRowA11y{}
	for _, n := range nodes {
		byLabel[n.label] = n
	}
	if n := byLabel["West (125)"]; n == nil || n.AccessibleState()&core.StateExpanded == 0 {
		t.Error("expanded group should report StateExpanded")
	}
	if n := byLabel["East (42)"]; n == nil || n.AccessibleState()&core.StateExpanded != 0 {
		t.Error("collapsed group must not report StateExpanded")
	}
}

// The overlay translates rows by the shared vertical scroll offset and skips
// rows scrolled out of its bounds.
func TestGroupRowOverlay_ScrollOffset(t *testing.T) {
	test.NewTempApp(t)
	g := &fakeGrid{
		rows: 100, cols: 1,
		groupRows: map[int]fakeGroupInfo{0: {key: "West", count: 5, expanded: true}},
	}
	vp := newOverlayViewport(g, 400, 300)

	w := NewGroupRowOverlay(g, vp)
	r := w.CreateRenderer().(*groupRowOverlayRenderer)
	r.refresh(geo.NewSize(400, 300))
	if len(overlayContent(r)) == 0 {
		t.Fatal("group row 0 should render at offset 0")
	}

	// Scroll the body down past the group row: the viewport window moves and
	// the overlay's offset shifts rows up out of its bounds.
	vp.Update(geo.NewPos(0, 1500), geo.NewSize(400, 300))
	w.offsetY = 1500
	r.refresh(geo.NewSize(400, 300))
	if n := len(overlayContent(r)); n != 0 {
		t.Fatalf("scrolled-out group row still drew %d objects", n)
	}
}

// The cell-band renderers no longer draw group-header content of their own:
// one label per group row total, owned by the grid-level overlay (#110).
func TestCellsAndPinnedRenderers_SkipGroupRows(t *testing.T) {
	test.NewTempApp(t)
	g := &fakeGrid{
		rows: 10, cols: 2, pinnedLeft: 1,
		groupRows: map[int]fakeGroupInfo{0: {key: "West", count: 3, expanded: true}},
	}
	vp := newOverlayViewport(g, 400, 300)

	// Main cells renderer: no group label text, no group a11y node, no hosts
	// on the group row.
	cw := NewCellsWidget(g, vp, NewCellPool())
	cr := NewCellsRenderer(cw)
	cr.Refresh()
	for _, o := range cr.objects {
		if txt, ok := o.(*canvas.Text); ok && strings.Contains(txt.Text, "West") {
			t.Errorf("CellsRenderer drew group label %q; overlay owns it", txt.Text)
		}
		if _, ok := o.(*groupRowA11y); ok {
			t.Error("CellsRenderer emitted a groupRowA11y node; overlay owns it")
		}
	}
	for _, h := range collectHosts(cr.objects) {
		if h.row == 0 {
			t.Errorf("CellsRenderer realized a cell host on group row 0 (col %d)", h.col)
		}
	}

	// Pinned renderer: same — the band draws only the row background.
	pw := NewPinnedCellsWidget(g, PinnedLeft, NewCellPool(), vp)
	pw.Resize(geo.NewSize(100, 300))
	pr := test.WidgetRenderer(pw).(*pinnedCellsRenderer)
	pr.Refresh()
	for _, o := range pr.objects {
		if txt, ok := o.(*canvas.Text); ok && strings.Contains(txt.Text, "West") {
			t.Errorf("pinned renderer drew group label %q; overlay owns it", txt.Text)
		}
	}
	for _, h := range pr.visible {
		if h.row == 0 {
			t.Errorf("pinned renderer realized a cell host on group row 0 (col %d)", h.col)
		}
	}
}

// A group row's aggregates are drawn in their own columns — pinned-left fixed,
// main translated by the horizontal scroll inside the main clip, pinned-right
// fixed — and appended to the row's accessible label (#689).
func TestGroupRowOverlay_Aggregates(t *testing.T) {
	test.NewTempApp(t)
	g := &fakeGrid{
		rows: 20, cols: 2, pinnedLeft: 1, pinnedRt: 1, scrollX: 30,
		groupRows: map[int]fakeGroupInfo{
			0: {key: "West", count: 3, expanded: true},
			4: {key: "East", count: 2, expanded: false},
		},
		groupAggs: map[int][]GroupAggregate{
			0: {
				{Col: 0, Text: "7", Label: "ID count 7"},
				{Col: 2, Text: "300", Label: "Salary sum 300"},
				{Col: 3, Text: "42.5", Label: "Score average 42.5"},
			},
			4: {{Col: 2, Text: "90", Label: "Salary sum 90"}},
		},
	}
	vp := newOverlayViewport(g, 400, 300)
	w := NewGroupRowOverlay(g, vp)
	size := geo.NewSize(600, 300)
	r := w.CreateRenderer().(*groupRowOverlayRenderer)
	r.refresh(size)

	texts := map[string]*canvas.Text{}
	inMainClip := map[string]bool{}
	var nodes []*groupRowA11y
	for _, o := range overlayContent(r) {
		switch v := o.(type) {
		case *canvas.Text:
			texts[v.Text] = v
		case *groupRowA11y:
			nodes = append(nodes, v)
		}
	}
	if r.mainAgg == nil {
		t.Fatal("no main-band aggregate clip was built")
	}
	for _, o := range r.mainAgg.Objects {
		if v, ok := o.(*canvas.Text); ok {
			texts[v.Text] = v
			inMainClip[v.Text] = true
		}
	}

	// Columns are 100 wide with padding 2; aggregates are inset by 4.
	// mainX = 102, pinnedRightX = 600 - 100 = 500, scroll 30.
	cases := []struct {
		text   string
		x, y   float32
		inMain bool
	}{
		{"7", 4, 2, false},             // pinned-left col 0, row 0
		{"300", 102 - 30 + 4, 2, true}, // main col 2, after main col 1 (100 + 2): clip-local x = 102 - 30
		{"42.5", 500 + 4, 2, false},    // pinned-right col 3
		{"90", 102 - 30 + 4, 4*32 + 2, true},
	}
	for _, c := range cases {
		tx := texts[c.text]
		if tx == nil {
			t.Errorf("aggregate %q not drawn", c.text)
			continue
		}
		if tx.Position() != geo.NewPos(c.x, c.y) {
			t.Errorf("aggregate %q at %v, want (%v, %v)", c.text, tx.Position(), c.x, c.y)
		}
		if inMainClip[c.text] != c.inMain {
			t.Errorf("aggregate %q in main clip = %v, want %v", c.text, inMainClip[c.text], c.inMain)
		}
	}
	if r.mainClip.Position().X != 102 || r.mainClip.Size().Width != 500-102 {
		t.Errorf("main clip x=%v w=%v, want x=102 w=398", r.mainClip.Position().X, r.mainClip.Size().Width)
	}

	labels := map[string]bool{}
	for _, n := range nodes {
		labels[n.AccessibleLabel()] = true
	}
	for _, want := range []string{
		"West (3), ID count 7, Salary sum 300, Score average 42.5",
		"East (2), Salary sum 90",
	} {
		if !labels[want] {
			t.Errorf("no group node labelled %q; got %v", want, labels)
		}
	}
}
