package tables

// columnLayout computes the visible column layout from resolved columns,
// separating them into pinned-left, main, and pinned-right groups.
type columnLayout struct {
	pinnedLeft  []*resolvedColumn
	main        []*resolvedColumn
	pinnedRight []*resolvedColumn
	all         []*resolvedColumn // all visible columns in display order (left + main + right)
	groups      []columnGroup     // top-level column groups
}

// columnGroup represents a group header spanning multiple child columns.
type columnGroup struct {
	def       *ColumnDef
	startCol  int  // index into layout.all
	endCol    int  // exclusive
	collapsed bool // true if group is collapsed (children hidden)
	depth     int  // nesting depth (0 = top-level)
}

// computeLayout builds the column layout from resolved columns.
func computeLayout(cols []*resolvedColumn) *columnLayout {
	layout := &columnLayout{}

	for _, col := range cols {
		if col.def.Hidden {
			continue
		}
		switch col.def.Pinned {
		case PinLeft:
			layout.pinnedLeft = append(layout.pinnedLeft, col)
		case PinRight:
			layout.pinnedRight = append(layout.pinnedRight, col)
		default:
			layout.main = append(layout.main, col)
		}
	}

	// Build display order: pinned-left, main, pinned-right
	layout.all = make([]*resolvedColumn, 0, len(layout.pinnedLeft)+len(layout.main)+len(layout.pinnedRight))
	layout.all = append(layout.all, layout.pinnedLeft...)
	layout.all = append(layout.all, layout.main...)
	layout.all = append(layout.all, layout.pinnedRight...)

	// Assign display indices
	for i, col := range layout.all {
		col.displayIndex = i
	}

	return layout
}

// pinnedLeftWidth returns the total width of pinned-left columns.
func (l *columnLayout) pinnedLeftWidth(padding float32) float32 {
	return columnsWidth(l.pinnedLeft, padding)
}

// pinnedRightWidth returns the total width of pinned-right columns.
func (l *columnLayout) pinnedRightWidth(padding float32) float32 {
	return columnsWidth(l.pinnedRight, padding)
}

func columnsWidth(cols []*resolvedColumn, padding float32) float32 {
	if len(cols) == 0 {
		return 0
	}
	w := float32(0)
	for _, col := range cols {
		w += col.width + padding
	}
	return w - padding // don't add padding after last column
}

// visibleColumnCount returns the number of visible columns.
func (l *columnLayout) visibleColumnCount() int {
	return len(l.all)
}

// columnWidths returns widths in display order.
func (l *columnLayout) columnWidths() []float32 {
	widths := make([]float32, len(l.all))
	for i, col := range l.all {
		widths[i] = col.width
	}
	return widths
}

// MoveColumn moves a column from one display position to another.
func (g *DataGrid) MoveColumn(fromIndex, toIndex int) {
	if fromIndex == toIndex {
		return
	}
	if fromIndex < 0 || fromIndex >= len(g.resolvedCols) {
		return
	}
	if toIndex < 0 || toIndex >= len(g.resolvedCols) {
		return
	}

	col := g.resolvedCols[fromIndex]

	// Check if column is movable
	movable := true
	if col.def.Movable != nil {
		movable = *col.def.Movable
	}
	if !movable {
		return
	}

	// Remove from old position
	copy(g.resolvedCols[fromIndex:], g.resolvedCols[fromIndex+1:])
	g.resolvedCols = g.resolvedCols[:len(g.resolvedCols)-1]

	// toIndex is the desired final position in the result.
	// After removal, the array is 1 shorter. Clamp insertAt.
	insertAt := toIndex
	if insertAt > len(g.resolvedCols) {
		insertAt = len(g.resolvedCols)
	}

	// Insert at new position
	g.resolvedCols = append(g.resolvedCols, nil) // grow
	copy(g.resolvedCols[insertAt+1:], g.resolvedCols[insertAt:])
	g.resolvedCols[insertAt] = col

	g.updateViewportGeometry()
	g.Refresh()
}

// SetColumnVisible shows or hides a column by its ID.
func (g *DataGrid) SetColumnVisible(colID string, visible bool) {
	for _, col := range g.resolvedCols {
		if col.def.ID() == colID {
			col.def.Hidden = !visible
			break
		}
	}
	g.updateViewportGeometry()
	g.Refresh()
}

// SetColumnPinned pins or unpins a column.
func (g *DataGrid) SetColumnPinned(colID string, side PinSide) {
	for _, col := range g.resolvedCols {
		if col.def.ID() == colID {
			col.def.Pinned = side
			break
		}
	}
	g.updateViewportGeometry()
	g.Refresh()
}

// VisibleColumns returns the column layout with pinned/main/right separation.
func (g *DataGrid) visibleLayout() *columnLayout {
	return computeLayout(g.resolvedCols)
}

// VisibleLayoutDebug returns [pinnedLeftCount, mainCount, pinnedRightCount] for debugging.
func (g *DataGrid) VisibleLayoutDebug() [3]int {
	l := g.visibleLayout()
	return [3]int{len(l.pinnedLeft), len(l.main), len(l.pinnedRight)}
}

// colSpanAt returns the column span for a cell, defaulting to 1.
func (g *DataGrid) colSpanAt(row, col int) int {
	rc := g.visibleCol(col)
	if rc == nil || rc.def.ColSpan == nil {
		return 1
	}
	span := rc.def.ColSpan(row)
	if span < 1 {
		return 1
	}
	layout := g.visibleLayout()
	remaining := len(layout.all) - col
	if span > remaining {
		span = remaining
	}
	return span
}

// --- Column Groups ---

// collapsedGroups tracks which column groups are collapsed.
// Key is the group column def's ID.
type collapsedGroupsMap map[string]bool

// ToggleColumnGroup toggles the expand/collapse state of a column group.
func (g *DataGrid) ToggleColumnGroup(groupID string) {
	if g.collapsedGroups == nil {
		g.collapsedGroups = make(collapsedGroupsMap)
	}
	g.collapsedGroups[groupID] = !g.collapsedGroups[groupID]
	g.resolveColumns()
	g.updateViewportGeometry()
	g.Refresh()
}

// IsGroupCollapsed returns true if the given group is collapsed.
func (g *DataGrid) IsGroupCollapsed(groupID string) bool {
	if g.collapsedGroups == nil {
		return false
	}
	return g.collapsedGroups[groupID]
}

// ExpandAllGroups expands all column groups.
func (g *DataGrid) ExpandAllGroups() {
	g.collapsedGroups = nil
	g.resolveColumns()
	g.updateViewportGeometry()
	g.Refresh()
}

// CollapseAllGroups collapses all column groups.
func (g *DataGrid) CollapseAllGroups() {
	if g.collapsedGroups == nil {
		g.collapsedGroups = make(collapsedGroupsMap)
	}
	g.collapseGroupsRecursive(g.ColumnDefs)
	g.resolveColumns()
	g.updateViewportGeometry()
	g.Refresh()
}

func (g *DataGrid) collapseGroupsRecursive(defs []*ColumnDef) {
	for _, def := range defs {
		if def.IsGroup() {
			g.collapsedGroups[def.ID()] = true
			g.collapseGroupsRecursive(def.Children)
		}
	}
}
