package tables

import (
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// statusBarText returns the text the grid's status bar draws, and whether it
// draws its background, read from the bar's renderer as the window shows it.
func statusBarText(t *testing.T, g *DataGrid) (text string, hasBg bool) {
	t.Helper()
	r := test.TempWidgetRenderer(t, g).(*dataGridRenderer)
	if r.statusBar == nil {
		t.Fatal("the grid has no status bar")
	}
	for _, o := range test.TempWidgetRenderer(t, r.statusBar).Objects() {
		switch o := o.(type) {
		case *canvas.Text:
			text = o.Text
		case *canvas.Rectangle:
			hasBg = true
		}
	}
	return text, hasBg
}

// TestDataGrid_StatusBarDrawsOnFirstShow pins #1500: the status bar drew
// nothing (no background, no row count) until something refreshed the grid,
// because its renderer built its objects only in Refresh.
func TestDataGrid_StatusBarDrawsOnFirstShow(t *testing.T) {
	data := []map[string]any{{"v": 1}, {"v": 2}, {"v": 3}, {"v": 4}, {"v": 5}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})
	g.Options.ShowStatusBar = true

	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))

	text, hasBg := statusBarText(t, g)
	if !hasBg {
		t.Error("the status bar draws no background when first shown")
	}
	if text != "5 rows" {
		t.Errorf("the status bar reads %q when first shown, want %q", text, "5 rows")
	}
}
