package render

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// disclosureIcon returns the arrow for an expanded (pointing down) or
// collapsed (pointing right) disclosure control: a group header, tree node or
// master-detail row. It is the theme's own icon, so it has the same style and
// colour as every other icon an app shows: the foreground colour, or the
// disabled colour while the grid is disabled. A text arrow went through font
// fallback instead, where bellui's bundled colour emoji font drew ▶ as a blue
// play button (#1256).
func disclosureIcon(expanded, disabled bool) bellui.Resource {
	res := theme.MenuExpandIcon()
	if expanded {
		res = theme.MenuDropDownIcon()
	}
	if disabled {
		return theme.NewDisabledResource(res)
	}
	return res
}

// disclosureSize is the height of a disclosure arrow's box. The theme's
// triangles fill only the middle of their square, so the box is larger than
// the text arrow it replaced, to draw an arrow of about the same size.
const disclosureSize float32 = 18

// newDisclosureImage makes an image for disclosureIcon, scaled to fit its box.
func newDisclosureImage() *canvas.Image {
	img := &canvas.Image{FillMode: canvas.ImageFillContain}
	return img
}

// setDisclosure shows the arrow for a state, changing the image's resource
// only when the state changed: a new resource makes the painter rasterize the
// icon again.
func setDisclosure(img *canvas.Image, expanded, disabled bool) {
	res := disclosureIcon(expanded, disabled)
	if img.Resource != nil && img.Resource.Name() == res.Name() {
		return
	}
	img.Resource = res
	img.Refresh()
}

// PinnedSide indicates which side this pinned region is on.
type PinnedSide int

const (
	PinnedLeft PinnedSide = iota
	PinnedRight
)

// PinnedCellsWidget renders cells for pinned columns.
// It is positioned outside the main scroll area and only scrolls vertically.
type PinnedCellsWidget struct {
	widget.BaseWidget

	Grid     GridAccess
	Side     PinnedSide
	pool     *CellPool
	viewport *Viewport // shared viewport (for row positions)
	offsetY  float32   // vertical scroll offset, synced from main scroll
}

// NewPinnedCellsWidget creates a pinned cells widget.
func NewPinnedCellsWidget(grid GridAccess, side PinnedSide, pool *CellPool, viewport *Viewport) *PinnedCellsWidget {
	w := &PinnedCellsWidget{
		Grid:     grid,
		Side:     side,
		pool:     pool,
		viewport: viewport,
	}
	w.ExtendBaseWidget(w)
	return w
}

// SetOffsetY updates the vertical scroll offset.
func (w *PinnedCellsWidget) SetOffsetY(y float32) {
	w.offsetY = y
	w.Refresh()
}

func (w *PinnedCellsWidget) CreateRenderer() core.WidgetRenderer {
	w.ExtendBaseWidget(w)
	return &pinnedCellsRenderer{
		widget:   w,
		visible:  make(map[cellKey]*cellHost),
		hostPool: map[string][]*cellHost{},
	}
}

type pinnedCellsRenderer struct {
	widget  *PinnedCellsWidget
	visible map[cellKey]*cellHost
	// hostPool recycles accessible cell hosts by pinned column type name, so a
	// pinned cell scrolling back into view reuses its host (and the content it
	// owns) instead of allocating. Mirrors CellsRenderer's host pool.
	hostPool map[string][]*cellHost
	objects  []core.CanvasObject

	// The rows are drawn at full height inside a real clip, so a row cut by
	// the band's edge is clipped rather than squeezed: its text stays level
	// with the same row in the main band. The band's bottom edge can sit above
	// the main scroll's horizontal bar (#1257), not only at the body's edge.
	band *bellui.Container
	clip *container.Clip

	// Disclosure decorations for the first pinned-left column, pooled so the
	// invisible a11y node keeps a stable identity across refreshes (unlike the
	// row backgrounds / group labels, which are cheap and recreated each pass).
	// Tree and master-detail are mutually exclusive, but keep independent pools
	// for clarity.
	treeChevrons   []*canvas.Image
	treeA11y       []*treeRowA11y
	masterChevrons []*canvas.Image
	masterA11y     []*masterRowA11y
}

// obtainPinnedHost returns a recycled host for the given pinned column type, or
// a fresh one wrapping newly created pinned content.
func (r *pinnedCellsRenderer) obtainPinnedHost(typeName string, displayCol int) *cellHost {
	if hosts := r.hostPool[typeName]; len(hosts) > 0 {
		h := hosts[len(hosts)-1]
		r.hostPool[typeName] = hosts[:len(hosts)-1]
		return h
	}
	return newCellHost(r.widget.Grid, r.widget.Grid.PinnedCreateCell(displayCol))
}

func (r *pinnedCellsRenderer) releasePinnedHost(typeName string, h *cellHost) {
	r.hostPool[typeName] = append(r.hostPool[typeName], h)
}

func (r *pinnedCellsRenderer) Layout(size geo.Size) {
	r.refresh(size)
}

func (r *pinnedCellsRenderer) MinSize() geo.Size {
	return geo.NewSize(0, 0)
}

func (r *pinnedCellsRenderer) Refresh() {
	r.refresh(r.widget.Size())
}

func (r *pinnedCellsRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *pinnedCellsRenderer) Destroy() {
	r.visible = nil
	r.hostPool = nil
}

func (r *pinnedCellsRenderer) refresh(size geo.Size) {
	grid := r.widget.Grid
	vp := r.widget.viewport
	offsetY := r.widget.offsetY

	// Determine which absolute display columns this region covers.
	// These are indices into the full visible column list (pinned + main + right).
	pinnedLeftCount := grid.PinnedLeftCount()
	pinnedRightCount := grid.PinnedRightCount()
	totalVisible := pinnedLeftCount + grid.ColumnCount() + pinnedRightCount

	var colStart, colEnd int
	switch r.widget.Side {
	case PinnedLeft:
		colStart = 0
		colEnd = pinnedLeftCount
	case PinnedRight:
		colStart = totalVisible - pinnedRightCount
		colEnd = totalVisible
	}

	if colStart >= colEnd {
		r.objects = nil
		return
	}

	// Pinned columns share the body's vertical scroll, so reuse the viewport's
	// visible range (variable-row-height aware) rather than assuming uniform
	// heights. Positions below already go through vp.RowY.
	rowCount := grid.RowCount()
	firstRow := vp.VisibleRows.Min
	lastRow := vp.VisibleRows.Max
	if firstRow < 0 {
		firstRow = 0
	}
	if lastRow >= rowCount {
		lastRow = rowCount - 1
	}

	tc := resolveColors(grid)
	altColor := tc.RowAlternate
	selColor := tc.Selection

	newVisible := make(map[cellKey]*cellHost)
	objects := make([]core.CanvasObject, 0, (lastRow-firstRow+1)*(colEnd-colStart)+16)

	// Get ALL visible column widths (for absolute display index lookup)
	widths := grid.HeaderColumnWidths()
	padding := grid.CellPadding()

	// The tree/master-detail disclosure lives on the first visible column; in a
	// PinnedLeft region that is absolute column 0 (colStart == 0).
	discCol := r.widget.Side == PinnedLeft && colStart == 0
	treeChevIdx, treeA11yIdx := 0, 0
	masterChevIdx, masterA11yIdx := 0, 0

	for row := firstRow; row <= lastRow; row++ {
		y, rh := vp.RowY(row)
		// An expanded master row's detail band spans only the main content
		// width; pinned cells stay at the cell-band height above it.
		rh -= grid.DetailBandHeight(row)
		y -= offsetY // adjust for scroll

		// Skip rows entirely outside the widget bounds; the clip cuts the
		// rest at the edges.
		if y+rh <= 0 || y >= size.Height {
			continue
		}
		visY, visH := y, rh

		// Row background
		rowBg := canvas.NewRectangle(tc.Background)
		if grid.IsAlternateRow(row) {
			rowBg.FillColor = altColor
		}
		if grid.IsSelected(row, colStart) {
			rowBg.FillColor = selColor
		}
		if grid.IsGroupRow(row) {
			// Shows in the strip the GroupRowOverlay leaves for the scroll bars.
			rowBg.FillColor = tc.GroupRowBg
		}
		rowBg.Move(geo.NewPos(0, visY))
		rowBg.Resize(geo.NewSize(size.Width, visH))
		objects = append(objects, rowBg)

		// Group row: no cells in the pinned band. The label and accessible node
		// are drawn once at the DataGrid level by the GroupRowOverlay spanning
		// all bands — a per-band copy here would collide with the main copy at
		// the band boundary, since canvas.Text does not clip (#110).
		if grid.IsGroupRow(row) {
			continue
		}

		// Data cells — use Pinned* methods with absolute display indices. Each
		// cell is wrapped in a cellHost so pinned columns carry the same grid
		// a11y semantics (RoleTableCell + label + absolute position + selection)
		// as main cells. col is already an absolute display index.
		x := float32(0)
		for col := colStart; col < colEnd; col++ {
			key := cellKey{row, col}
			typeName := grid.PinnedColumnTypeName(col)

			host, exists := r.visible[key]
			if exists {
				delete(r.visible, key)
			} else {
				host = r.obtainPinnedHost(typeName, col)
			}
			host.row, host.col = row, col // absolute display coordinates

			colW := float32(100)
			if col < len(widths) {
				colW = widths[col]
			}

			// Tree / master-detail disclosure on the first pinned-left column:
			// indent the cell content and draw a chevron + invisible a11y node
			// for disclosable rows. Tree and master-detail are mutually exclusive.
			discIndent := float32(0)
			if discCol && col == 0 {
				switch {
				case grid.IsTreeRow(row):
					depth, hasChildren, expanded := grid.TreeRowInfo(row)
					discIndent = grid.TreeFirstColIndent(row)
					if hasChildren {
						depthX := x + float32(depth)*TreeIndentPerLevel

						if treeChevIdx >= len(r.treeChevrons) {
							r.treeChevrons = append(r.treeChevrons, newDisclosureImage())
						}
						chev := r.treeChevrons[treeChevIdx]
						treeChevIdx++
						setDisclosure(chev, expanded, gridDisabled(grid))
						chev.Move(geo.NewPos(depthX+2, visY+(visH-disclosureSize)/2))
						chev.Resize(geo.NewSize(TreeToggleWidth, disclosureSize))
						objects = append(objects, chev)

						if treeA11yIdx >= len(r.treeA11y) {
							r.treeA11y = append(r.treeA11y, newTreeRowA11y(grid))
						}
						ta := r.treeA11y[treeA11yIdx]
						treeA11yIdx++
						ta.set(row, grid.AccessibleCellText(row, col), depth+1, expanded)
						ta.Move(geo.NewPos(depthX, visY))
						ta.Resize(geo.NewSize(TreeToggleWidth+8, visH))
						objects = append(objects, ta)
					}

				case grid.DetailToggleWidth() > 0:
					// Master-detail active: reserve the indent on every row for
					// alignment; draw the chevron + a11y only on master rows.
					discIndent = grid.DetailToggleWidth()
					if grid.IsMasterRow(row) {
						expanded := grid.DetailBandHeight(row) > 0

						if masterChevIdx >= len(r.masterChevrons) {
							r.masterChevrons = append(r.masterChevrons, newDisclosureImage())
						}
						chev := r.masterChevrons[masterChevIdx]
						masterChevIdx++
						setDisclosure(chev, expanded, gridDisabled(grid))
						chev.Move(geo.NewPos(x+4, visY+(visH-disclosureSize)/2))
						chev.Resize(geo.NewSize(discIndent, disclosureSize))
						objects = append(objects, chev)

						if masterA11yIdx >= len(r.masterA11y) {
							r.masterA11y = append(r.masterA11y, newMasterRowA11y(grid))
						}
						ma := r.masterA11y[masterA11yIdx]
						masterA11yIdx++
						ma.set(row, grid.AccessibleCellText(row, col), expanded)
						ma.Move(geo.NewPos(x, visY))
						ma.Resize(geo.NewSize(discIndent+8, visH))
						objects = append(objects, ma)
					}
				}
			}

			grid.PinnedUpdateCell(row, col, host.content)
			host.Move(geo.NewPos(x+discIndent, visY))
			host.Resize(geo.NewSize(colW-discIndent, visH))

			newVisible[key] = host
			objects = append(objects, host)

			x += colW + padding
		}
	}

	// Recycle hosts that scrolled out of view.
	for key, host := range r.visible {
		typeName := grid.PinnedColumnTypeName(key.col)
		r.releasePinnedHost(typeName, host)
	}
	r.visible = newVisible
	if r.band == nil {
		r.band = container.NewWithoutLayout()
		r.clip = container.NewClip(r.band)
	}
	r.band.Objects = objects
	r.clip.Move(geo.NewPos(0, 0))
	r.clip.Resize(size)
	r.clip.Refresh()
	r.objects = []core.CanvasObject{r.clip}
	// See CellsRenderer.refreshCells (#758). MatchScope does not descend into
	// containers, and skips the clip once it carries the scope, so the rows
	// inside it are matched themselves.
	theme.MatchScope(r.widget, r.clip)
	theme.MatchScope(r.widget, objects...)
}
