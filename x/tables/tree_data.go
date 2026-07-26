package tables

import "github.com/belluikit/bellui/x/tables/render"

// Tree data turns flat rows into a parent/child hierarchy via a
// pipeline.TreeTransform (mirroring how grouping uses pipeline.GroupTransform).
// Unlike master-detail — which overlays a detail band on a real row and lives
// outside the pipeline — tree rows ARE data rows: the transform reorders and
// hides them by expansion, and the first main column carries a depth indent plus
// a disclosure chevron for parent nodes. Expansion state is keyed by node ID on
// the persistent transform, so it survives re-sort/re-filter like grouping.
//
// The chevron/indent are rendered on the first MAIN (non-pinned) column; a tree
// therefore wants its hierarchy column left of the pinned split (typically the
// first non-pinned column). See render.TreeIndentPerLevel / render.TreeToggleWidth
// for the shared geometry.

// treeActive reports whether tree data is configured.
func (g *DataGrid) treeActive() bool {
	return g.treeTransform != nil
}

// IsTreeRow reports whether the given display row is a tree node (tree data
// active and the row index is within the transform's node list). Tree rows are
// real data rows, so unlike group headers they always have a source identity.
func (g *DataGrid) IsTreeRow(row int) bool {
	if g.treeTransform == nil {
		return false
	}
	// TreeNodes is built over the full pre-paginate output; a visible display row
	// is offset into it by the current page (see pageRowOffset).
	row += g.pageRowOffset()
	return row >= 0 && row < len(g.treeTransform.TreeNodes)
}

// TreeRowInfo returns the depth (0 = root), whether the node has children, and
// whether it is currently expanded, for the given display row.
func (g *DataGrid) TreeRowInfo(row int) (depth int, hasChildren, expanded bool) {
	if !g.IsTreeRow(row) {
		return 0, false, false
	}
	n := g.treeTransform.TreeNodes[row+g.pageRowOffset()]
	return n.Depth, n.HasChildren, n.Expanded
}

// TreeFirstColIndent returns the left indent applied to the first main column of
// the given display row: the depth indentation plus the chevron slot. It is the
// single source of truth shared by the renderer (content shift) and the chevron
// hit-test. Zero when tree data is inactive or the row is not a tree node.
func (g *DataGrid) TreeFirstColIndent(row int) float32 {
	if !g.IsTreeRow(row) {
		return 0
	}
	depth, _, _ := g.TreeRowInfo(row)
	return float32(depth)*render.TreeIndentPerLevel + render.TreeToggleWidth
}

// treeNodeID resolves a display row to its tree node ID (via GetID on the source
// row data), or nil if the row has no source identity.
func (g *DataGrid) treeNodeID(displayRow int) any {
	if g.treeTransform == nil || g.treeTransform.GetID == nil {
		return nil
	}
	src := g.sourceRow(displayRow)
	if src < 0 {
		return nil
	}
	return g.treeTransform.GetID(g.RowData(src))
}

// ToggleTreeNode expands or collapses the tree node at the given DISPLAY row and
// rebuilds the pipeline so the change is reflected. Rows without children are
// ignored. It is the single tree-toggle entry point shared by the public
// GridAPI, the pointer hit-test, the keyboard handler, and the accessible
// expand/collapse action (see render.treeRowA11y).
func (g *DataGrid) ToggleTreeNode(displayRow int) {
	if g.treeTransform == nil {
		return
	}
	if _, hasChildren, _ := g.TreeRowInfo(displayRow); !hasChildren {
		return
	}
	id := g.treeNodeID(displayRow)
	if id == nil {
		return
	}
	g.treeTransform.ToggleNode(id)
	g.rebuildPipeline()
}

// toggleFocusedTreeNode expands/collapses the tree node when the keyboard cursor
// is on a tree parent row, routing through the same ToggleTreeNode path as a
// chevron click and the accessible action. It returns true when it handled a
// tree parent, so the Enter/Space key does not also attempt a cell selection.
func (g *DataGrid) toggleFocusedTreeNode() bool {
	if !g.IsTreeRow(g.currentFocus.Row) {
		return false
	}
	if _, hasChildren, _ := g.TreeRowInfo(g.currentFocus.Row); !hasChildren {
		return false
	}
	g.ToggleTreeNode(g.currentFocus.Row)
	return true
}
