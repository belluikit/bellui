// Command a11yprobe is a live-verification harness for fytables' DataGrid
// accessibility (issues #15/#16), mirroring the fyne fork's hack/a11yprobe
// pattern. It puts a single grid on screen with everything the grid a11y work
// touches — pinned columns (left + right), row grouping (group-header rows),
// and far more rows than fit, so the view virtualizes — then leaves the window
// open for an external driver to inspect the real platform accessibility tree.
//
// This is a separate module (its own go.mod) so the GL/app dependency tree stays
// out of fytables' library go.mod; it builds against the parent checkout via a
// replace directive. Build and run from this directory (the accessibility tag
// activates the fork's UIA / AX backend):
//
//	cd hack/a11yprobe
//	go build -tags accessibility -o a11yprobe.exe .
//	./a11yprobe.exe                 # default: #15/#16 model (no checkbox column)
//	./a11yprobe.exe -checkbox       # #31: leading checkbox-selection column
//	./a11yprobe.exe -masterdetail   # #45: expandable master-detail rows
//	./a11yprobe.exe -tree           # #47: hierarchical tree data (RoleTreeItem)
//
// On Windows, drive it with the fork's UIA clients (uiatree.ps1 <pid>) and the
// NVDA speech-capture harness; see hack/a11yprobe/windows/ in this repo and the
// fork's hack/a11yprobe/README.md.
//
// What the platform tree should show:
//   - a Table element whose Grid pattern reports RowCount = the full display-row
//     count (hundreds, incl. group headers) × ColumnCount = 5 (id + name +
//     region + score + status, i.e. pinned-left + 3 main + pinned-right),
//     while only the on-screen cells are materialized as DataItem elements —
//     the virtualization contract of #16 and the absolute column model of #15;
//   - DataItem (cell) elements exposing a GridItem pattern with row/column in
//     that absolute space, for both main and pinned columns (#15);
//   - Group elements for the region group-header rows, carrying an
//     ExpandCollapse pattern (#15). With -checkbox, invoking that pattern's
//     Collapse()/Expand() toggles the group (routed through ToggleGroup) — the
//     action half of #31 — and works the same without the flag.
//
// With -checkbox, the model gains a leading checkbox-selection column at
// absolute index 0, so the Grid pattern reports 6 columns, every DataItem's
// GridItem column shifts one place right (pinned-left -> 1 … pinned-right -> 5),
// and a DataItem at column 0 wraps an interactive CheckBox; the pre-selected
// rows' column-0 cells report SelectionItem selected=True (#29/#31).
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/tables"
)

// rowCount is deliberately far larger than the visible window so the grid
// virtualizes: the Table's reported RowCount dwarfs the count of materialized
// DataItem cells.
const rowCount = 500

var regions = []string{"North", "South", "East", "West"}

// treeFanout controls the shape of the tree-mode dataset: treeRoots roots, each
// with treeChildren children, each of those with treeGrandchildren leaves — a
// 3-level tree so AccessibleLevel varies (root = 1 … leaf = 3). The total is kept
// near rowCount so the grid still virtualizes.
const (
	treeRoots         = 20
	treeChildren      = 5
	treeGrandchildren = 4
)

// Data is []map[string]any (not a struct) because fytables' default value-getter
// reads map keys; a struct would resolve every field to nil (and collapse
// grouping into one nil group).
func makeData() []map[string]any {
	statuses := []string{"open", "ack", "resolved"}
	data := make([]map[string]any, rowCount)
	for i := range data {
		data[i] = map[string]any{
			"id":     1000 + i,
			"name":   fmt.Sprintf("Problem %03d", i),
			"region": regions[i%len(regions)],
			"score":  (i * 7) % 100,
			"status": statuses[i%len(statuses)],
		}
	}
	return data
}

// makeTreeData builds a 3-level hierarchy as []map[string]any with "id" and
// "parent" keys (parent = -1 for roots), consumed by SetTreeData's GetID/GetParent.
// Row order is intentionally NOT pre-sorted into tree order: the TreeTransform
// reconstructs the hierarchy from the parent links, exercising that path.
func makeTreeData() []map[string]any {
	statuses := []string{"open", "ack", "resolved"}
	var data []map[string]any
	nextID := 0
	add := func(parent int, label, region string) int {
		id := nextID
		nextID++
		data = append(data, map[string]any{
			"id":     id,
			"parent": parent,
			"name":   label,
			"region": region,
			"score":  (id * 7) % 100,
			"status": statuses[id%len(statuses)],
		})
		return id
	}
	for r := 0; r < treeRoots; r++ {
		region := regions[r%len(regions)]
		root := add(-1, fmt.Sprintf("Root %02d", r), region)
		for c := 0; c < treeChildren; c++ {
			child := add(root, fmt.Sprintf("Root %02d / Child %d", r, c), region)
			for g := 0; g < treeGrandchildren; g++ {
				add(child, fmt.Sprintf("Root %02d / Child %d / Leaf %d", r, c, g), region)
			}
		}
	}
	return data
}

// tree turns the rows into a hierarchical tree (no grouping) and expands every
// node, so the platform tree exposes the tree-item disclosure semantics of #47
// (a RoleTreeItem carrying AccessibleLevel + the ExpandCollapse pattern /
// StateExpanded routed to ToggleTreeNode) on parent rows at multiple depths.
// Because the leftmost column ('id') is pinned-left, the disclosure renders on
// that pinned column — exercising the pinned-hierarchy path of #50.
var tree = flag.Bool("tree", false,
	"enable hierarchical tree data and expand all nodes (issue #47/#50)")

// checkbox enables the leading checkbox-selection column and pre-selects a
// couple of rows, so the platform tree exposes the checkbox-cell semantics of
// #29 (a RoleTableCell/DataItem at absolute column 0 carrying the row's
// selection state, with an interactive CheckBox inside it) that #31 validates.
var checkbox = flag.Bool("checkbox", false,
	"enable the checkbox-selection column and pre-select rows (issue #31)")

// masterDetail turns the rows into expandable master rows (no grouping) and
// expands a couple, so the platform tree exposes the master-row disclosure
// semantics of #45 (a Button carrying the ExpandCollapse pattern / StateExpanded
// routed to ToggleDetail).
var masterDetail = flag.Bool("masterdetail", false,
	"enable expandable master-detail rows and expand a couple (issue #45)")

func main() {
	flag.Parse()
	// The app must exist before the grid: DataGrid's render pipeline resolves
	// theme colors on configuration refreshes (render.ResolveThemeColors →
	// bellui.CurrentApp().Settings()), which nil-panics pre-app under fyne v3.
	a := app.New()
	data := makeData()
	if *tree {
		data = makeTreeData()
	}

	grid := tables.NewDataGrid(
		[]*tables.ColumnDef{
			{ColID: "id", Field: "id", HeaderName: "ID", Width: 70, Pinned: tables.PinLeft, Type: tables.ColumnNumber},
			{ColID: "name", Field: "name", HeaderName: "Name", Width: 160},
			{ColID: "region", Field: "region", HeaderName: "Region", Width: 110},
			{ColID: "score", Field: "score", HeaderName: "Score", Width: 90, Type: tables.ColumnNumber},
			{ColID: "status", Field: "status", HeaderName: "Status", Width: 110, Pinned: tables.PinRight},
		},
		func() int { return len(data) },
		func(row int) any { return data[row] },
	)
	// Name the grid so the Table element has a UIA/AX Name to key on.
	grid.Options.AccessibleLabel = "Problems"
	grid.Options.RowAlternation = true
	if *checkbox {
		grid.Options.CheckboxSelection = true
	}

	// Print PROBE lines so a driver can confirm an interaction reached the
	// widget, not just the bridge (mirrors the fork probe convention).
	grid.OnCellClicked = func(id tables.CellID) {
		fmt.Printf("PROBE cell-clicked row=%d col=%d\n", id.Row, id.Col)
	}
	grid.OnSelectionChanged = func(rows []int) {
		fmt.Printf("PROBE selection=%v\n", rows)
	}

	switch {
	case *tree:
		// Hierarchical tree data instead of grouping: parent rows disclose their
		// children with a RoleTreeItem + ExpandCollapse pattern. Expand all nodes
		// so the tree shows items at every depth (AccessibleLevel 1..3). The
		// disclosure renders on the pinned-left 'id' column (the leftmost visible
		// column), exercising #50.
		grid.API().SetTreeData(
			func(row any) any { return row.(map[string]any)["id"] },
			func(row any) int { return row.(map[string]any)["parent"].(int) },
		)
		grid.API().ExpandAllNodes()
	case *masterDetail:
		// Expandable master rows instead of grouping: each row can disclose a
		// detail band. Expand a couple so the tree shows both states.
		grid.MasterDetail = tables.MasterDetailConfig{
			Enabled:      true,
			DetailHeight: 60,
			DetailRenderer: func(sourceRow int) core.CanvasObject {
				return canvas.NewText(fmt.Sprintf("detail for row %d", sourceRow), nil)
			},
		}
		grid.ToggleDetail(0)
		grid.ToggleDetail(2)
	default:
		// Group by region so the tree carries group-header rows (RoleTableRow),
		// then expand them so their children are present.
		grid.API().SetRowGroupColumn("region")
		grid.API().ExpandAllGroups()
	}

	// Pre-select two "North" data rows (source rows 0 and 4 both fall in the
	// first, top-of-view group) so their checkbox cells report StateSelected in
	// the platform tree — the selection half of #31's checkbox check.
	if *checkbox {
		grid.API().SelectRow(0)
		grid.API().SelectRow(4)
	}

	w := a.NewWindow("fytables a11y probe")
	w.SetContent(grid)
	w.Resize(geo.NewSize(760, 460))

	fmt.Printf("PROBE ready pid=%d rows=%d regions=%d\n", os.Getpid(), len(data), len(regions))
	w.Show()
	a.Run()
}
