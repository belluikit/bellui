package tables

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// unthemed walks everything the grid renders and counts, by type, the objects
// whose theme is not the grid's own, along with how many objects it saw.
func unthemed(g *DataGrid) (missing map[string]int, total int) {
	want := theme.CurrentForWidget(g)
	missing = map[string]int{}
	seen := map[core.CanvasObject]bool{}
	var walk func(core.CanvasObject)
	walk = func(o core.CanvasObject) {
		if o == nil || seen[o] {
			return
		}
		seen[o] = true
		total++
		if theme.CurrentForWidget(o) != want {
			missing[fmt.Sprintf("%T", o)]++
		}
		switch v := o.(type) {
		case core.Widget:
			for _, c := range test.WidgetRenderer(v).Objects() {
				walk(c)
			}
		case *bellui.Container:
			for _, c := range v.Objects {
				walk(c)
			}
		}
	}
	walk(g)
	return missing, total
}

// The grid makes its rows, checkboxes and cell hosts at layout, after a theme
// override around it was applied, so each of them has to take the override's
// scope as it is made or reused from a pool (#758).
func TestDataGrid_ThemeOverrideReachesEverythingItRenders(t *testing.T) {
	rows := func(n int) []map[string]any {
		data := make([]map[string]any, n)
		for i := range data {
			data[i] = map[string]any{"n": fmt.Sprintf("row%d", i), "dept": fmt.Sprintf("d%d", i%7)}
		}
		return data
	}
	for _, tc := range []struct {
		name  string
		setup func(*DataGrid)
	}{
		{"checkboxes, pinned columns and floating filters", func(g *DataGrid) {
			g.Options.CheckboxSelection = true
			g.Options.FloatingFilters = true
			g.SelectionMode = SelectionMultiRow
		}},
		{"grouped rows", func(g *DataGrid) { g.API().SetRowGroupColumn("dept") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newTestGrid(t, rows(1000), []*ColumnDef{
				{ColID: "p", Field: "n", Pinned: PinLeft},
				{ColID: "dept", Field: "dept"},
				{ColID: "n", Field: "n"},
			})
			tc.setup(g)
			w := test.NewWindow(container.NewThemeOverride(g, test.NewTheme()))
			t.Cleanup(w.Close)
			require.NotEqual(t, theme.Current(), theme.CurrentForWidget(g), "the grid itself should carry the override")

			for _, step := range []struct {
				name string
				do   func()
			}{
				{"first layout", func() { w.Resize(geo.NewSize(300, 150)) }},
				{"grown", func() { w.Resize(geo.NewSize(300, 900)) }},
				{"shrunk", func() { w.Resize(geo.NewSize(300, 150)) }},
				{"grown again from the pools", func() { w.Resize(geo.NewSize(300, 900)) }},
				{"refreshed", func() { g.Refresh() }},
			} {
				step.do()
				missing, total := unthemed(g)
				var types []string
				n := 0
				for k, v := range missing {
					types = append(types, fmt.Sprintf("%s=%d", k, v))
					n += v
				}
				sort.Strings(types)
				assert.Zerof(t, n, "%s: %d of %d rendered objects draw with the app theme: %v", step.name, n, total, types)
				assert.Greater(t, total, 50, "%s: the walk should reach the rows", step.name)
			}
		})
	}
}
