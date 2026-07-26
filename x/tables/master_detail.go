package tables

import "github.com/belluikit/bellui/core"

// detailToggleWidth is the horizontal space reserved at the left of the first
// main column for the master-detail disclosure chevron. It doubles as the
// click target width for toggling expansion.
const detailToggleWidth float32 = 18

// MasterDetailConfig configures the master-detail feature.
type MasterDetailConfig struct {
	// Enabled activates master-detail mode.
	Enabled bool
	// DetailHeight is the height of the detail band rendered beneath an expanded
	// master row (in addition to the row's own cell height).
	DetailHeight float32
	// IsRowMaster reports whether the given SOURCE row can be expanded. When nil,
	// every row is treated as a master row.
	IsRowMaster func(sourceRow int) bool
	// DetailRenderer creates the detail content for an expanded SOURCE row. The
	// returned object is cached per source row until the data is refreshed, so it
	// may carry its own state.
	DetailRenderer func(sourceRow int) core.CanvasObject
}

// expandedDetails tracks which SOURCE rows have their detail expanded. Keying by
// source identity (rather than display position) keeps expansion attached to the
// underlying data across sort/filter/paginate — consistent with selection, the
// keyboard cursor, and per-row heights.
type expandedDetails map[int]bool

// masterDetailActive reports whether master-detail is fully configured and can
// contribute geometry and render surfaces.
func (g *DataGrid) masterDetailActive() bool {
	return g.MasterDetail.Enabled &&
		g.MasterDetail.DetailRenderer != nil &&
		g.MasterDetail.DetailHeight > 0
}

// isRowMaster reports whether the given source row can be expanded.
func (g *DataGrid) isRowMaster(sourceRow int) bool {
	if sourceRow < 0 {
		return false
	}
	if g.MasterDetail.IsRowMaster == nil {
		return true
	}
	return g.MasterDetail.IsRowMaster(sourceRow)
}

// ToggleDetail toggles the detail expansion for the given DISPLAY row. The row is
// resolved to its source identity so the expansion survives re-sort/filter.
func (g *DataGrid) ToggleDetail(displayRow int) {
	g.toggleDetailSource(g.sourceRow(displayRow))
}

// toggleDetailSource toggles expansion for a source row, rebuilds geometry, and
// refreshes. Non-master rows are ignored.
func (g *DataGrid) toggleDetailSource(sourceRow int) {
	if !g.isRowMaster(sourceRow) {
		return
	}
	if g.expandedDetails == nil {
		g.expandedDetails = make(expandedDetails)
	}
	if g.expandedDetails[sourceRow] {
		delete(g.expandedDetails, sourceRow)
		delete(g.detailContent, sourceRow) // drop cached content on collapse
	} else {
		g.expandedDetails[sourceRow] = true
	}
	g.Refresh()
}

// IsDetailExpanded reports whether the given DISPLAY row's detail is expanded.
func (g *DataGrid) IsDetailExpanded(displayRow int) bool {
	return g.isSourceExpanded(g.sourceRow(displayRow))
}

// isSourceExpanded reports whether a source row's detail is expanded.
func (g *DataGrid) isSourceExpanded(sourceRow int) bool {
	if sourceRow < 0 || g.expandedDetails == nil {
		return false
	}
	return g.expandedDetails[sourceRow]
}

// ExpandAllDetails expands every master row's detail.
func (g *DataGrid) ExpandAllDetails() {
	if !g.masterDetailActive() {
		return
	}
	g.expandedDetails = make(expandedDetails)
	for src := 0; src < g.Length(); src++ {
		if g.isRowMaster(src) {
			g.expandedDetails[src] = true
		}
	}
	g.Refresh()
}

// CollapseAllDetails collapses all detail rows.
func (g *DataGrid) CollapseAllDetails() {
	g.expandedDetails = make(expandedDetails)
	g.detailContent = nil
	g.Refresh()
}
