package tables

import (
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// newMasterDetailGrid builds a rendered grid with master-detail configured.
func newMasterDetailGrid(t *testing.T, data []map[string]any, cfg MasterDetailConfig) *DataGrid {
	t.Helper()
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})
	g.MasterDetail = cfg
	w := test.NewWindow(g)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 400))
	return g
}

func defaultDetailCfg(detailH float32) MasterDetailConfig {
	return MasterDetailConfig{
		Enabled:      true,
		DetailHeight: detailH,
		DetailRenderer: func(sourceRow int) core.CanvasObject {
			return canvas.NewText("detail", nil)
		},
	}
}

// Enter and Space on a focused master row toggle its detail band, like a click
// on the disclosure chevron (#45).
func TestMasterDetail_KeyboardTogglesFocusedDetail(t *testing.T) {
	data := []map[string]any{{"name": "A"}, {"name": "B"}}
	g := newMasterDetailGrid(t, data, defaultDetailCfg(60))

	if !g.IsMasterRow(0) {
		t.Fatal("expected row 0 to be a master row")
	}
	g.currentFocus = CellID{Row: 0, Col: 0}
	g.focusSrcRow = g.sourceRow(0)

	// Enter expands the detail.
	g.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
	if !g.IsDetailExpanded(0) {
		t.Fatal("Enter on a master row did not expand its detail")
	}

	// Space collapses it again.
	g.TypedKey(&event.KeyEvent{Name: event.KeySpace})
	if g.IsDetailExpanded(0) {
		t.Fatal("Space on a master row did not collapse its detail")
	}
}

// Expanding a row adds DetailHeight to its viewport row height and grows the
// total content height by the same amount.
func TestMasterDetail_ExpandAddsBandHeight(t *testing.T) {
	data := []map[string]any{{"name": "A"}, {"name": "B"}, {"name": "C"}}
	g := newMasterDetailGrid(t, data, defaultDetailCfg(60))
	def := g.rowHeight()
	pad := g.cellPadding()

	before := g.viewport.ContentSize().Height
	if g.viewport.RowHeights != nil {
		t.Fatalf("expected no overrides while collapsed, got %v", g.viewport.RowHeights)
	}

	g.ToggleDetail(1)

	rh := g.viewport.RowHeights
	if len(rh) != 1 || rh[1] != def+60 {
		t.Fatalf("expected display row 1 height %v, got %v", def+60, rh)
	}
	if _, h := g.viewport.RowY(1); h != def+60 {
		t.Errorf("RowY(1) height = %v, want %v", h, def+60)
	}
	// Row 2 now starts below the expanded row 1.
	wantRow2Y := (def + pad) + (def + 60 + pad)
	if y, _ := g.viewport.RowY(2); y != wantRow2Y {
		t.Errorf("RowY(2).y = %v, want %v", y, wantRow2Y)
	}
	if got := g.viewport.ContentSize().Height; got != before+60 {
		t.Errorf("content height = %v, want %v", got, before+60)
	}
}

// Expansion is keyed by source identity, so a tall detail row follows its data
// across a sort.
func TestMasterDetail_ExpansionSurvivesSort(t *testing.T) {
	data := []map[string]any{{"name": "Charlie"}, {"name": "Alice"}, {"name": "Bob"}}
	g := newMasterDetailGrid(t, data, defaultDetailCfg(50))
	def := g.rowHeight()

	// Expand Charlie (source 0), currently display row 0.
	g.ToggleDetail(0)
	if g.viewport.RowHeights[0] != def+50 {
		t.Fatalf("pre-sort: expected expanded display row 0, got %v", g.viewport.RowHeights)
	}

	// Sort ascending: Charlie moves to display row 2.
	g.API().SetSort(SortItem{ColID: "name", Direction: SortAscending})

	rh := g.viewport.RowHeights
	if len(rh) != 1 || rh[2] != def+50 {
		t.Errorf("post-sort: expected expanded display row 2, got %v", rh)
	}
	if !g.IsDetailExpanded(2) {
		t.Errorf("post-sort: display row 2 (Charlie) should read as expanded")
	}
	if g.IsDetailExpanded(0) {
		t.Errorf("post-sort: display row 0 (Alice) should not be expanded")
	}
}

// Collapsing every row returns the viewport to the uniform fast path.
func TestMasterDetail_CollapseReturnsToUniform(t *testing.T) {
	data := []map[string]any{{"name": "A"}, {"name": "B"}}
	g := newMasterDetailGrid(t, data, defaultDetailCfg(40))

	g.ToggleDetail(0)
	if g.viewport.RowHeights == nil {
		t.Fatal("expected overrides after expand")
	}
	g.ToggleDetail(0)
	if g.viewport.RowHeights != nil {
		t.Errorf("expected uniform fast path after collapse, got %v", g.viewport.RowHeights)
	}
}

// DetailContent is created lazily and cached per source row; collapsing drops it.
func TestMasterDetail_ContentCaching(t *testing.T) {
	data := []map[string]any{{"name": "A"}, {"name": "B"}}
	calls := 0
	cfg := MasterDetailConfig{
		Enabled:      true,
		DetailHeight: 40,
		DetailRenderer: func(sourceRow int) core.CanvasObject {
			calls++
			return canvas.NewText("d", nil)
		},
	}
	g := newMasterDetailGrid(t, data, cfg)

	if c := g.DetailContent(0); c != nil {
		t.Fatalf("collapsed row should have no detail content, got %v", c)
	}
	g.ToggleDetail(0)

	first := g.DetailContent(0)
	second := g.DetailContent(0)
	if first == nil || first != second {
		t.Fatalf("expected a single cached content object, got %v and %v", first, second)
	}
	if calls != 1 {
		t.Errorf("DetailRenderer called %d times, want 1", calls)
	}

	// Collapsing drops the cache so a re-expand rebuilds fresh content.
	g.ToggleDetail(0)
	g.ToggleDetail(0)
	if g.DetailContent(0) == first {
		t.Errorf("expected fresh content after collapse+expand")
	}
}

// The disclosure chevron hit-zone toggles the row; taps elsewhere do not.
func TestMasterDetail_ChevronHitZoneToggles(t *testing.T) {
	data := []map[string]any{{"name": "A"}, {"name": "B"}}
	g := newMasterDetailGrid(t, data, defaultDetailCfg(40))
	headerH := g.HeaderHeight()

	// A press within the left chevron indent of row 0.
	if row := g.detailToggleAt(geo.NewPos(6, headerH+5)); row != 0 {
		t.Fatalf("chevron zone hit = %d, want row 0", row)
	}
	// A press well past the chevron indent is not a toggle.
	if row := g.detailToggleAt(geo.NewPos(120, headerH+5)); row != -1 {
		t.Errorf("data-cell area should not toggle, got row %d", row)
	}

	g.ToggleDetail(0)
	// Once expanded, a press in the detail band is not a chevron toggle.
	def := g.rowHeight()
	if row := g.detailToggleAt(geo.NewPos(6, headerH+def+10)); row != -1 {
		t.Errorf("detail band should not be a chevron zone, got row %d", row)
	}
}

// A tap inside an expanded detail band is not reported as a data cell.
func TestMasterDetail_CellAtPositionSkipsBand(t *testing.T) {
	data := []map[string]any{{"name": "A"}, {"name": "B"}}
	g := newMasterDetailGrid(t, data, defaultDetailCfg(50))
	headerH := g.HeaderHeight()
	def := g.rowHeight()

	g.ToggleDetail(0)

	// Inside the cell band of row 0 → a valid cell.
	if id := g.cellAtPosition(geo.NewPos(60, headerH+def/2)); id.Row != 0 {
		t.Errorf("cell band hit = %+v, want row 0", id)
	}
	// Inside the detail band of row 0 → no cell.
	if id := g.cellAtPosition(geo.NewPos(60, headerH+def+20)); id.Row != -1 || id.Col != -1 {
		t.Errorf("detail band hit = %+v, want {-1,-1}", id)
	}
}

// ExpandAllDetails honours IsRowMaster; CollapseAllDetails clears everything.
func TestMasterDetail_ExpandAllRespectsIsRowMaster(t *testing.T) {
	data := []map[string]any{{"name": "A"}, {"name": "B"}, {"name": "C"}}
	cfg := defaultDetailCfg(30)
	cfg.IsRowMaster = func(sourceRow int) bool { return sourceRow != 1 } // row 1 not a master
	g := newMasterDetailGrid(t, data, cfg)

	g.ExpandAllDetails()
	if len(g.expandedDetails) != 2 || g.expandedDetails[1] {
		t.Fatalf("expected rows 0,2 expanded, got %v", g.expandedDetails)
	}
	if g.IsMasterRow(1) {
		t.Errorf("row 1 should not be a master row")
	}

	// A non-master row cannot be toggled.
	g.ToggleDetail(1)
	if g.expandedDetails[1] {
		t.Errorf("toggling a non-master row should be a no-op")
	}

	g.CollapseAllDetails()
	if len(g.expandedDetails) != 0 {
		t.Errorf("expected all collapsed, got %v", g.expandedDetails)
	}
}

// The detail content object is materialised into the rendered scene when a row
// is expanded, and removed once collapsed.
func TestMasterDetail_ContentRendered(t *testing.T) {
	data := []map[string]any{{"name": "A"}, {"name": "B"}}
	marker := canvas.NewText("DETAIL-MARK", nil)
	cfg := MasterDetailConfig{
		Enabled:      true,
		DetailHeight: 40,
		DetailRenderer: func(sourceRow int) core.CanvasObject {
			if sourceRow == 0 {
				return marker
			}
			return canvas.NewText("other", nil)
		},
	}
	g := newMasterDetailGrid(t, data, cfg)

	inScene := func() bool {
		r := test.WidgetRenderer(g.cellsWidget)
		for _, o := range r.Objects() {
			if o == marker {
				return true
			}
		}
		return false
	}

	if inScene() {
		t.Fatal("detail marker present before expansion")
	}
	g.ToggleDetail(0)
	if !inScene() {
		t.Error("detail marker missing after expansion")
	}
	g.ToggleDetail(0)
	if inScene() {
		t.Error("detail marker still present after collapse")
	}
}

// With master-detail inactive, none of the geometry/hit hooks engage.
func TestMasterDetail_InactiveIsInert(t *testing.T) {
	data := []map[string]any{{"name": "A"}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})
	// Enabled but no renderer / no height ⇒ inactive.
	g.MasterDetail = MasterDetailConfig{Enabled: true}
	if g.masterDetailActive() {
		t.Fatal("expected inactive without renderer/height")
	}
	if g.DetailToggleWidth() != 0 {
		t.Errorf("toggle width should be 0 when inactive")
	}
	if g.DetailBandHeight(0) != 0 || g.IsMasterRow(0) {
		t.Errorf("no band or master rows when inactive")
	}
}
