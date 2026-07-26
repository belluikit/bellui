package render

import (
	"fmt"
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// cellKey identifies a visible cell.
type cellKey struct {
	row, col int
}

// checkboxCellWidth is the rendered width of the per-row select control inside
// the leading checkbox-selection column. Shared by the realized renderer and
// the off-screen virtual-node provider so both report the same cell geometry.
const checkboxCellWidth = 30

// Tree-data layout. TreeIndentPerLevel is the horizontal indentation added to a
// tree row's first main column per depth level; TreeToggleWidth is the
// disclosure-chevron slot that follows that indent (and doubles as the chevron
// click target). Exported so the root package's geometry (first-column indent
// and the chevron hit-test) shares one source of truth with the renderer.
const (
	TreeIndentPerLevel float32 = 16
	TreeToggleWidth    float32 = 16
)

// CellsRenderer manages the visible cell CanvasObjects.
type CellsRenderer struct {
	cells   *CellsWidget
	visible map[cellKey]core.CanvasObject // values are *cellHost
	objects []core.CanvasObject

	// hostPool recycles accessible cell hosts (and the content each owns) by
	// column type name, so main cells bypass the shared cell pool used by
	// pinned cells and each content stays 1:1 with its host.
	hostPool map[string][]*cellHost

	rowBgs      []*canvas.Rectangle
	groupLabels []*groupRowWidget
	groupA11y   []*groupRowA11y
	checkboxes  []*widget.Check
	// checkboxHosts wraps each pooled select checkbox in a cellHost so assistive
	// tech sees a RoleTableCell (at absolute column 0) carrying the row's
	// selection state, with the interactive widget.Check reachable inside it.
	checkboxHosts []*cellHost
	detailBgs     []*canvas.Rectangle
	chevrons      []*canvas.Text
	masterA11y    []*masterRowA11y
	treeChevrons  []*canvas.Text
	treeA11y      []*treeRowA11y
}

type groupRowWidget struct {
	bg      *canvas.Rectangle
	icon    *canvas.Text
	label   *canvas.Text
	objects []core.CanvasObject
}

func newGroupRowWidget(tc ThemeColors) *groupRowWidget {
	bg := canvas.NewRectangle(tc.GroupRowBg)
	icon := canvas.NewText("▼", tc.GroupRowFg)
	icon.TextSize = 11
	label := canvas.NewText("", tc.GroupRowFg)
	label.TextStyle = bellui.TextStyle{Bold: true}
	label.TextSize = 13
	return &groupRowWidget{bg: bg, icon: icon, label: label, objects: []core.CanvasObject{bg, icon, label}}
}

func (g *groupRowWidget) update(key string, childCount int, expanded bool, depth int, pos geo.Position, size geo.Size, tc ThemeColors) {
	indent := float32(depth) * 20
	g.bg.FillColor = tc.GroupRowBg
	g.bg.Move(pos)
	g.bg.Resize(size)
	if expanded {
		g.icon.Text = "▼"
	} else {
		g.icon.Text = "▶"
	}
	g.icon.Color = tc.GroupRowFg
	g.icon.Move(geo.NewPos(pos.X+indent+4, pos.Y+2))
	g.icon.Resize(geo.NewSize(14, size.Height-4))
	g.label.Text = fmt.Sprintf("%s (%d)", key, childCount)
	g.label.Color = tc.GroupRowFg
	g.label.Move(geo.NewPos(pos.X+indent+20, pos.Y+2))
	g.label.Resize(geo.NewSize(size.Width-indent-24, size.Height-4))
}

// groupRowA11y is an invisible accessible node overlaid on a group-header row so
// assistive tech announces it as a RoleTableRow carrying the group label and its
// expanded/collapsed state. The row's visuals are drawn separately as raw canvas
// objects (groupRowWidget); this widget contributes only semantics. It is not
// Tappable/Hoverable, so the DataGrid's geometry hit-testing still handles the
// toggle.
type groupRowA11y struct {
	widget.BaseWidget
	grid     GridAccess
	key      string
	label    string
	expanded bool
}

var (
	_ core.Accessible         = (*groupRowA11y)(nil)
	_ core.AccessibleStateful = (*groupRowA11y)(nil)
	_ core.AccessibleActioner = (*groupRowA11y)(nil)
)

func newGroupRowA11y(grid GridAccess) *groupRowA11y {
	g := &groupRowA11y{grid: grid}
	g.ExtendBaseWidget(g)
	return g
}

func (g *groupRowA11y) set(key string, childCount int, expanded bool) {
	g.key = key
	g.label = fmt.Sprintf("%s (%d)", key, childCount)
	g.expanded = expanded
}

func (g *groupRowA11y) CreateRenderer() core.WidgetRenderer {
	g.ExtendBaseWidget(g)
	// A transparent rectangle gives the node bounds without drawing anything.
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (g *groupRowA11y) AccessibleRole() core.AccessibleRole { return core.RoleTableRow }
func (g *groupRowA11y) AccessibleLabel() string             { return g.label }

// AccessibleState reports StateExpanded when the group is open (there is no
// distinct "collapsed" state — its absence means collapsed).
func (g *groupRowA11y) AccessibleState() core.AccessibleState {
	if g.expanded {
		return core.StateExpanded
	}
	return 0
}

// AccessibleActions exposes the one relevant expand/collapse action for the
// group's current state, so the UIA ExpandCollapse pattern's Expand()/Collapse()
// map onto it (mirroring how a tree branch advertises its actions).
func (g *groupRowA11y) AccessibleActions() []core.AccessibleAction {
	if g.expanded {
		return []core.AccessibleAction{{Name: core.ActionCollapse, Description: "Collapse group"}}
	}
	return []core.AccessibleAction{{Name: core.ActionExpand, Description: "Expand group"}}
}

// PerformAccessibleAction toggles the group when the requested action matches
// its current state, routing through the same ToggleGroup path a pointer tap on
// the group header uses. It is a no-op when the group is already in the target
// state (expand while expanded, collapse while collapsed).
func (g *groupRowA11y) PerformAccessibleAction(name string) {
	if g.grid == nil {
		return
	}
	switch name {
	case core.ActionExpand:
		if !g.expanded {
			g.grid.ToggleGroup(g.key)
		}
	case core.ActionCollapse:
		if g.expanded {
			g.grid.ToggleGroup(g.key)
		}
	}
}

// masterRowA11y is an invisible accessible node overlaid on a master row's
// disclosure area so assistive tech can announce and toggle its expandable
// detail band. A master row already exposes its data as RoleTableCell cells;
// this adds a Button carrying the ExpandCollapse pattern (StateExpanded + one
// expand/collapse action routed to ToggleDetail), mirroring groupRowA11y for
// group headers. It is a Button (not a RoleTableRow) so it reads as a disclosure
// control and does not compete with the cells the platform groups into the row.
// Not Tappable/Hoverable — the grid's geometry hit-testing still handles the
// chevron click.
type masterRowA11y struct {
	widget.BaseWidget
	grid     GridAccess
	row      int
	label    string
	expanded bool
}

var (
	_ core.Accessible         = (*masterRowA11y)(nil)
	_ core.AccessibleStateful = (*masterRowA11y)(nil)
	_ core.AccessibleActioner = (*masterRowA11y)(nil)
)

func newMasterRowA11y(grid GridAccess) *masterRowA11y {
	m := &masterRowA11y{grid: grid}
	m.ExtendBaseWidget(m)
	return m
}

func (m *masterRowA11y) set(row int, rowLabel string, expanded bool) {
	m.row = row
	if rowLabel != "" {
		m.label = rowLabel + " details"
	} else {
		m.label = "Row details"
	}
	m.expanded = expanded
}

func (m *masterRowA11y) CreateRenderer() core.WidgetRenderer {
	m.ExtendBaseWidget(m)
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (m *masterRowA11y) AccessibleRole() core.AccessibleRole { return core.RoleButton }
func (m *masterRowA11y) AccessibleLabel() string             { return m.label }

// AccessibleState reports StateExpanded when the detail band is open.
func (m *masterRowA11y) AccessibleState() core.AccessibleState {
	if m.expanded {
		return core.StateExpanded
	}
	return 0
}

// AccessibleActions exposes the one relevant expand/collapse action for the
// row's current disclosure state (mirroring groupRowA11y and tree branches).
func (m *masterRowA11y) AccessibleActions() []core.AccessibleAction {
	if m.expanded {
		return []core.AccessibleAction{{Name: core.ActionCollapse, Description: "Collapse row detail"}}
	}
	return []core.AccessibleAction{{Name: core.ActionExpand, Description: "Expand row detail"}}
}

// PerformAccessibleAction toggles the detail band when the requested action
// matches its current state, routing through the same ToggleDetail path a click
// on the disclosure chevron uses.
func (m *masterRowA11y) PerformAccessibleAction(name string) {
	if m.grid == nil {
		return
	}
	switch name {
	case core.ActionExpand:
		if !m.expanded {
			m.grid.ToggleDetail(m.row)
		}
	case core.ActionCollapse:
		if m.expanded {
			m.grid.ToggleDetail(m.row)
		}
	}
}

// treeRowA11y is an invisible accessible node overlaid on a tree parent row's
// disclosure area so assistive tech announces and toggles its child rows. Unlike
// master-detail (a Button) and group headers (a RoleTableRow), a tree node reads
// as a RoleTreeItem carrying its nesting level (AccessibleLevel) plus the
// ExpandCollapse pattern (StateExpanded + one expand/collapse action routed to
// ToggleTreeNode). Only parent rows (HasChildren) get one — leaf rows expose
// their data as ordinary RoleTableCell cells. Not Tappable/Hoverable; the grid's
// geometry hit-testing still handles the chevron click.
type treeRowA11y struct {
	widget.BaseWidget
	grid     GridAccess
	row      int
	label    string
	level    int
	expanded bool
}

var (
	_ core.Accessible         = (*treeRowA11y)(nil)
	_ core.AccessibleStateful = (*treeRowA11y)(nil)
	_ core.AccessibleActioner = (*treeRowA11y)(nil)
	_ core.AccessibleLeveler  = (*treeRowA11y)(nil)
)

func newTreeRowA11y(grid GridAccess) *treeRowA11y {
	t := &treeRowA11y{grid: grid}
	t.ExtendBaseWidget(t)
	return t
}

// set records the row's identity and disclosure state. level is 1-based (a root
// node is level 1), matching core.AccessibleLeveler; the renderer passes depth+1.
func (t *treeRowA11y) set(row int, rowLabel string, level int, expanded bool) {
	t.row = row
	t.label = rowLabel
	t.level = level
	t.expanded = expanded
}

func (t *treeRowA11y) CreateRenderer() core.WidgetRenderer {
	t.ExtendBaseWidget(t)
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (t *treeRowA11y) AccessibleRole() core.AccessibleRole { return core.RoleTreeItem }
func (t *treeRowA11y) AccessibleLabel() string             { return t.label }

// AccessibleLevel returns the 1-based nesting level so AT can announce tree depth.
func (t *treeRowA11y) AccessibleLevel() int { return t.level }

// AccessibleState reports StateExpanded when the node is open (its absence means
// collapsed).
func (t *treeRowA11y) AccessibleState() core.AccessibleState {
	if t.expanded {
		return core.StateExpanded
	}
	return 0
}

// AccessibleActions exposes the one relevant expand/collapse action for the
// node's current state, so the UIA ExpandCollapse pattern maps onto it.
func (t *treeRowA11y) AccessibleActions() []core.AccessibleAction {
	if t.expanded {
		return []core.AccessibleAction{{Name: core.ActionCollapse, Description: "Collapse tree node"}}
	}
	return []core.AccessibleAction{{Name: core.ActionExpand, Description: "Expand tree node"}}
}

// PerformAccessibleAction toggles the node when the requested action matches its
// current state, routing through the same ToggleTreeNode path a chevron click and
// the keyboard handler use.
func (t *treeRowA11y) PerformAccessibleAction(name string) {
	if t.grid == nil {
		return
	}
	switch name {
	case core.ActionExpand:
		if !t.expanded {
			t.grid.ToggleTreeNode(t.row)
		}
	case core.ActionCollapse:
		if t.expanded {
			t.grid.ToggleTreeNode(t.row)
		}
	}
}

// NewCellsRenderer creates a renderer for the CellsWidget.
func NewCellsRenderer(cells *CellsWidget) *CellsRenderer {
	return &CellsRenderer{
		cells:   cells,
		visible: make(map[cellKey]core.CanvasObject),
	}
}

func (r *CellsRenderer) Layout(size geo.Size) { r.refreshCells() }

func (r *CellsRenderer) MinSize() geo.Size {
	return r.cells.viewport.ContentSize()
}

func (r *CellsRenderer) Refresh() { r.refreshCells() }

func (r *CellsRenderer) Objects() []core.CanvasObject { return r.objects }

func (r *CellsRenderer) Destroy() {
	r.visible = nil
	r.hostPool = nil
}

// obtainHost returns a recycled accessible cell host for the given column type,
// or a fresh one wrapping a newly created content object. Content stays 1:1
// with its host for the host's lifetime (mirroring fyne's tableCellHost), so
// main cells never touch the shared cell pool that pinned cells use.
func (r *CellsRenderer) obtainHost(typeName string, col int) *cellHost {
	if hosts := r.hostPool[typeName]; len(hosts) > 0 {
		h := hosts[len(hosts)-1]
		r.hostPool[typeName] = hosts[:len(hosts)-1]
		return h
	}
	return newCellHost(r.cells.Grid, r.cells.Grid.CreateCell(col))
}

// releaseHost returns a host (and the content it owns) to the pool for its
// column type so it can be reused when that cell scrolls back into view.
func (r *CellsRenderer) releaseHost(typeName string, h *cellHost) {
	if r.hostPool == nil {
		r.hostPool = map[string][]*cellHost{}
	}
	r.hostPool[typeName] = append(r.hostPool[typeName], h)
}

func (r *CellsRenderer) refreshCells() {
	grid := r.cells.Grid
	vp := r.cells.viewport
	tc := ResolveThemeColors()

	visRows := vp.VisibleRows
	visCols := vp.VisibleCols

	// Guard: nothing to render if no visible rows or columns.
	if visRows.Max < visRows.Min || visCols.Max < visCols.Min ||
		grid.ColumnCount() == 0 || grid.RowCount() == 0 {
		r.objects = nil
		return
	}

	showCheckbox := grid.ShowCheckboxColumn()
	// Main columns occupy absolute display indices after the pinned-left block;
	// cell hosts report a11y coordinates in that absolute space (see cellHost).
	pinnedLeftCount := grid.PinnedLeftCount()

	estCells := max(0, (visRows.Max-visRows.Min+1)*(visCols.Max-visCols.Min+1))
	newVisible := make(map[cellKey]core.CanvasObject, estCells)
	objects := make([]core.CanvasObject, 0, estCells+64)

	groupIdx := 0
	bgIdx := 0
	cbIdx := 0
	detailIdx := 0
	chevIdx := 0
	mdIdx := 0
	treeChevIdx := 0
	treeA11yIdx := 0
	toggleW := grid.DetailToggleWidth()

	for row := visRows.Min; row <= visRows.Max; row++ {
		y, rowH := vp.RowY(row)
		totalW := vp.ContentSize().Width

		// An expanded master row's full height splits into a cell band (top,
		// cellH) that hosts the data cells and a detail band (bottom, bandH).
		bandH := grid.DetailBandHeight(row)
		cellH := rowH - bandH

		// Row background (cell band only)
		if bgIdx >= len(r.rowBgs) {
			r.rowBgs = append(r.rowBgs, canvas.NewRectangle(color.Transparent))
		}
		rowBg := r.rowBgs[bgIdx]
		bgIdx++

		rowBg.FillColor = tc.Background
		if grid.IsAlternateRow(row) {
			rowBg.FillColor = tc.RowAlternate
		}
		if grid.IsSelected(row, 0) {
			rowBg.FillColor = tc.Selection
		}
		rowBg.Move(geo.NewPos(0, y))
		rowBg.Resize(geo.NewSize(totalW, cellH))
		rowBg.Show()
		objects = append(objects, rowBg)

		// Detail band: a background panel plus the consumer's detail content.
		if bandH > 0 {
			if detailIdx >= len(r.detailBgs) {
				r.detailBgs = append(r.detailBgs, canvas.NewRectangle(color.Transparent))
			}
			db := r.detailBgs[detailIdx]
			detailIdx++
			db.FillColor = tc.RowAlternate
			db.Move(geo.NewPos(0, y+cellH))
			db.Resize(geo.NewSize(totalW, bandH))
			db.Show()
			objects = append(objects, db)

			if content := grid.DetailContent(row); content != nil {
				content.Move(geo.NewPos(0, y+cellH))
				content.Resize(geo.NewSize(totalW, bandH))
				content.Show()
				objects = append(objects, content)
			}
		}

		// Group header row
		if grid.IsGroupRow(row) {
			if groupIdx >= len(r.groupLabels) {
				r.groupLabels = append(r.groupLabels, newGroupRowWidget(tc))
				r.groupA11y = append(r.groupA11y, newGroupRowA11y(grid))
			}
			gw := r.groupLabels[groupIdx]
			ga := r.groupA11y[groupIdx]
			groupIdx++
			key, childCount, expanded, depth := grid.GroupRowInfo(row)
			gw.update(key, childCount, expanded, depth, geo.NewPos(0, y), geo.NewSize(totalW, cellH), tc)
			objects = append(objects, gw.objects...)
			// Accessible RoleTableRow overlay (semantics only, invisible).
			ga.set(key, childCount, expanded)
			ga.Move(geo.NewPos(0, y))
			ga.Resize(geo.NewSize(totalW, rowH))
			objects = append(objects, ga)
			continue
		}

		// Checkbox column — the interactive widget.Check is wrapped in a cellHost
		// so AT sees a RoleTableCell at absolute column 0 (the leading select
		// column) carrying the row's selection state, with the check reachable
		// inside it. The host's sentinel column -1 maps to reported column 0.
		checkboxW := float32(0)
		if showCheckbox {
			if cbIdx >= len(r.checkboxes) {
				cb := widget.NewCheck("", nil)
				host := newCellHost(grid, cb)
				host.col = -1
				r.checkboxes = append(r.checkboxes, cb)
				r.checkboxHosts = append(r.checkboxHosts, host)
			}
			cb := r.checkboxes[cbIdx]
			host := r.checkboxHosts[cbIdx]
			cbIdx++
			cb.SetChecked(grid.IsSelected(row, 0))
			host.row = row
			host.Move(geo.NewPos(0, y))
			host.Resize(geo.NewSize(checkboxCellWidth, cellH))
			objects = append(objects, host)
			checkboxW = 32
		}

		// Disclosure chevron for master rows, in the reserved left indent.
		// Rendered here only when the first visible column is a main column; with
		// pinned-left columns the chevron moves to the first pinned column (see
		// pinned_cells.go), mirroring the tree disclosure.
		if toggleW > 0 && grid.IsMasterRow(row) && pinnedLeftCount == 0 {
			if chevIdx >= len(r.chevrons) {
				t := canvas.NewText("", tc.GroupRowFg)
				t.TextSize = 11
				r.chevrons = append(r.chevrons, t)
			}
			chev := r.chevrons[chevIdx]
			chevIdx++
			if bandH > 0 {
				chev.Text = "▼"
			} else {
				chev.Text = "▶"
			}
			chev.Color = tc.GroupRowFg
			chev.Move(geo.NewPos(checkboxW+4, y+(cellH-14)/2))
			chev.Resize(geo.NewSize(toggleW, 14))
			objects = append(objects, chev)

			// Accessible disclosure control (semantics only, invisible), read
			// before the row's cells. Labelled by the row's first-column value.
			if mdIdx >= len(r.masterA11y) {
				r.masterA11y = append(r.masterA11y, newMasterRowA11y(grid))
			}
			ma := r.masterA11y[mdIdx]
			mdIdx++
			ma.set(row, grid.AccessibleCellText(row, 0), bandH > 0)
			ma.Move(geo.NewPos(checkboxW, y))
			ma.Resize(geo.NewSize(toggleW+8, cellH))
			objects = append(objects, ma)
		}

		// Disclosure chevron + a11y for tree parent rows. The chevron sits after
		// the row's depth indent, in its reserved slot before the first column's
		// content; leaf rows are indented but get no chevron. Rendered here only
		// when the first visible column is a main column; with pinned-left columns
		// the disclosure moves to the first pinned column (see pinned_cells.go).
		if grid.IsTreeRow(row) && pinnedLeftCount == 0 {
			depth, hasChildren, expanded := grid.TreeRowInfo(row)
			if hasChildren {
				if treeChevIdx >= len(r.treeChevrons) {
					t := canvas.NewText("", tc.GroupRowFg)
					t.TextSize = 11
					r.treeChevrons = append(r.treeChevrons, t)
				}
				chev := r.treeChevrons[treeChevIdx]
				treeChevIdx++
				if expanded {
					chev.Text = "▼"
				} else {
					chev.Text = "▶"
				}
				chev.Color = tc.GroupRowFg
				depthIndent := checkboxW + float32(depth)*TreeIndentPerLevel
				chev.Move(geo.NewPos(depthIndent+2, y+(cellH-14)/2))
				chev.Resize(geo.NewSize(TreeToggleWidth, 14))
				objects = append(objects, chev)

				// Accessible RoleTreeItem disclosure node (semantics only,
				// invisible), read before the row's cells. Labelled by the row's
				// first-column value; level is 1-based (root = 1).
				if treeA11yIdx >= len(r.treeA11y) {
					r.treeA11y = append(r.treeA11y, newTreeRowA11y(grid))
				}
				ta := r.treeA11y[treeA11yIdx]
				treeA11yIdx++
				ta.set(row, grid.AccessibleCellText(row, 0), depth+1, expanded)
				ta.Move(geo.NewPos(depthIndent, y))
				ta.Resize(geo.NewSize(TreeToggleWidth+8, cellH))
				objects = append(objects, ta)
			}
		}

		// Data cells — viewport only contains main (non-pinned) columns,
		// so render all visible viewport columns.
		col := visCols.Min
		for col <= visCols.Max {
			span := grid.ColSpanAt(row, col)
			key := cellKey{row, col}
			typeName := grid.ColumnTypeName(col)

			var host *cellHost
			if existing, exists := r.visible[key]; exists {
				host = existing.(*cellHost)
				delete(r.visible, key)
			} else {
				host = r.obtainHost(typeName, col)
			}
			// Rendering/pooling stay in main-column space (key, UpdateCell); the
			// host's a11y coordinates are absolute display order.
			host.row, host.col = row, pinnedLeftCount+col

			x, colW := vp.ColX(col)
			x += checkboxW
			spanW := colW
			for s := 1; s < span && col+s <= visCols.Max; s++ {
				_, sw := vp.ColX(col + s)
				spanW += sw + vp.Padding
			}

			// The first main column reserves a left indent for the disclosure
			// chevron (master-detail) or the tree depth + chevron (tree data),
			// so its content shifts right and shrinks to match.
			if col == 0 {
				if indent := r.firstColIndent(row); indent > 0 {
					x += indent
					spanW -= indent
				}
			}

			// UpdateCell fills the wrapped content (canvas.Text etc.); the host
			// only carries accessibility semantics, so it forwards the bare
			// content object.
			grid.UpdateCell(row, col, host.content)
			host.Move(geo.NewPos(x, y))
			host.Resize(geo.NewSize(spanW, cellH))

			newVisible[key] = host
			objects = append(objects, host)
			col += span
		}
	}

	for key, obj := range r.visible {
		typeName := grid.ColumnTypeName(key.col)
		r.releaseHost(typeName, obj.(*cellHost))
	}
	r.visible = newVisible

	for i := bgIdx; i < len(r.rowBgs); i++ {
		r.rowBgs[i].Hide()
	}
	for i := detailIdx; i < len(r.detailBgs); i++ {
		r.detailBgs[i].Hide()
	}

	r.objects = objects
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// firstColIndent returns the left indent reserved at the start of the given
// row's first main column: the tree depth + chevron slot for a tree row, else
// the master-detail disclosure width (uniform across rows when active). Tree and
// master-detail are alternative disclosure modes, so at most one is non-zero.
//
// Both disclosures live on the first VISIBLE column. When pinned-left columns
// exist that is a pinned column (rendered by PinnedCellsWidget), so the first
// MAIN column carries no indent here.
func (r *CellsRenderer) firstColIndent(row int) float32 {
	g := r.cells.Grid
	if g.PinnedLeftCount() > 0 {
		return 0
	}
	if g.IsTreeRow(row) {
		return g.TreeFirstColIndent(row)
	}
	return g.DetailToggleWidth()
}
