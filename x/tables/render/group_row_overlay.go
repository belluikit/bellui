package render

import (
	"fmt"
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
	"github.com/belluikit/bellui/x/tables/cell"
)

// GroupRowOverlay renders group-header rows at the DataGrid level, spanning the
// full body width — pinned-left band through the pinned-right overlay. Group
// headers used to be drawn once per horizontal region (main CellsWidget + each
// pinned band), which tripled the label and let the pinned-left copy collide
// with the main copy at the band boundary (canvas.Text does not clip, #110);
// the accessible node also lived in the main CellsWidget, so its bounds
// excluded the pinned zone and touch exploration over a pinned column found
// nothing (#111). Hoisting the row to one grid-level overlay fixes both: the
// label draws once, anchored at the grid's left edge (so it stays visible
// under horizontal scroll), and the invisible RoleTableRow node (groupRowA11y)
// spans the entire row.
//
// The overlay is positioned over the body area (below the header, above every
// cell band) and shares the body's vertical scroll via SetOffsetY, like the
// pinned widgets. It draws nothing over data rows and implements no pointer
// interfaces, so the DataGrid's geometry hit-testing keeps handling the
// expand/collapse tap and events over data rows fall through untouched.
type GroupRowOverlay struct {
	widget.BaseWidget

	Grid     GridAccess
	viewport *Viewport
	offsetY  float32 // vertical scroll offset, synced from the main scroll
}

// NewGroupRowOverlay creates the grid-level group-header overlay.
func NewGroupRowOverlay(grid GridAccess, viewport *Viewport) *GroupRowOverlay {
	w := &GroupRowOverlay{Grid: grid, viewport: viewport}
	w.ExtendBaseWidget(w)
	return w
}

// SetOffsetY updates the vertical scroll offset (mirroring the pinned bands).
func (w *GroupRowOverlay) SetOffsetY(y float32) {
	w.offsetY = y
	w.Refresh()
}

func (w *GroupRowOverlay) CreateRenderer() core.WidgetRenderer {
	w.ExtendBaseWidget(w)
	return &groupRowOverlayRenderer{widget: w}
}

type groupRowOverlayRenderer struct {
	widget  *GroupRowOverlay
	objects []core.CanvasObject

	// Pooled per visible group row. The a11y nodes keep a stable widget
	// identity across refreshes; the visuals ride along in the same pool.
	rows []*groupRowWidget
	a11y []*groupRowA11y

	// The overlay's content is wrapped in a real clip so a row half-scrolled
	// under the header cannot draw its label above the overlay (canvas.Text
	// does not clip on its own).
	band *bellui.Container
	clip *container.Clip

	// Aggregates (#689): pooled texts, and a clip for the ones in the
	// horizontally scrolling main band so they cannot draw over the pinned
	// bands. The main clip spans the overlay's height; its content is in
	// clip-local coordinates.
	aggTexts []*canvas.Text
	mainAgg  *bellui.Container
	mainClip *container.Clip
}

func (r *groupRowOverlayRenderer) Layout(size geo.Size) { r.refresh(size) }

func (r *groupRowOverlayRenderer) MinSize() geo.Size { return geo.NewSize(0, 0) }

func (r *groupRowOverlayRenderer) Refresh() { r.refresh(r.widget.Size()) }

func (r *groupRowOverlayRenderer) Objects() []core.CanvasObject { return r.objects }

func (r *groupRowOverlayRenderer) Destroy() {}

func (r *groupRowOverlayRenderer) refresh(size geo.Size) {
	grid := r.widget.Grid
	vp := r.widget.viewport
	if grid == nil || vp == nil || size.Width <= 0 || size.Height <= 0 {
		r.objects = nil
		return
	}

	tc := ResolveThemeColors()

	// Same visible range as the body; positions go through vp.RowY (variable
	// row heights) translated by the shared vertical scroll offset.
	rowCount := grid.RowCount()
	firstRow := vp.VisibleRows.Min
	lastRow := vp.VisibleRows.Max
	if firstRow < 0 {
		firstRow = 0
	}
	if lastRow >= rowCount {
		lastRow = rowCount - 1
	}

	geom := newAggGeometry(grid, size)
	var content, mainAggs []core.CanvasObject
	aggIdx := 0
	idx := 0
	for row := firstRow; row <= lastRow; row++ {
		if !grid.IsGroupRow(row) {
			continue
		}
		y, rowH := vp.RowY(row)
		y -= r.widget.offsetY
		rowH -= grid.DetailBandHeight(row)
		if y+rowH <= 0 || y >= size.Height {
			continue
		}

		if idx >= len(r.rows) {
			r.rows = append(r.rows, newGroupRowWidget(tc))
			r.a11y = append(r.a11y, newGroupRowA11y(grid))
		}
		gw := r.rows[idx]
		ga := r.a11y[idx]
		idx++

		key, childCount, expanded, depth := grid.GroupRowInfo(row)

		// Aggregates first: the label ends before the leftmost one that
		// reaches past its start, so the two never overlap (#748).
		labelEnd := size.Width
		labelX := groupLabelX(depth)
		var pinnedAggs []core.CanvasObject
		aggs := grid.GroupRowAggregates(row)
		for _, a := range aggs {
			x, w, inMain, ok := geom.column(a.Col)
			if !ok {
				continue
			}
			if aggIdx >= len(r.aggTexts) {
				r.aggTexts = append(r.aggTexts, newAggText())
			}
			t := r.aggTexts[aggIdx]
			aggIdx++
			placeAggText(t, a.Text, x, y, w, rowH, tc)
			if left, right, shown := geom.drawnExtent(t, inMain); shown && right > labelX && left < labelEnd {
				labelEnd = left
			}
			if inMain {
				mainAggs = append(mainAggs, t)
			} else {
				pinnedAggs = append(pinnedAggs, t)
			}
		}
		gw.update(key, childCount, expanded, depth, geo.NewPos(0, y), geo.NewSize(size.Width, rowH), labelEnd, tc)
		content = append(content, gw.objects...)
		content = append(content, pinnedAggs...)

		// Accessible RoleTableRow node (semantics only, invisible), spanning
		// the full row so touch exploration finds it over any band (#111).
		ga.set(key, childCount, expanded, aggs)
		ga.Move(geo.NewPos(0, y))
		ga.Resize(geo.NewSize(size.Width, rowH))
		content = append(content, ga)
	}

	if len(mainAggs) > 0 {
		if r.mainAgg == nil {
			r.mainAgg = container.NewWithoutLayout()
			r.mainClip = container.NewClip(r.mainAgg)
		}
		r.mainAgg.Objects = mainAggs
		r.mainClip.Move(geo.NewPos(geom.mainX, 0))
		r.mainClip.Resize(geo.NewSize(geom.mainRight-geom.mainX, size.Height))
		r.mainClip.Refresh()
		content = append(content, r.mainClip)
	}

	if r.band == nil {
		r.band = container.NewWithoutLayout()
	}
	r.band.Objects = content
	if r.clip == nil {
		r.clip = container.NewClip(r.band)
	}
	r.clip.Move(geo.NewPos(0, 0))
	r.clip.Resize(size)
	r.clip.Refresh()

	r.objects = []core.CanvasObject{r.clip}
	theme.MatchScope(r.widget, r.clip) // see CellsRenderer.refreshCells (#758)
}

// aggGeometry places a group row's aggregates in their columns using the same
// band anchors the header and body use (RegionLayoutAccess): pinned-left fixed
// from 0, main translated by -ScrollOffsetX and clipped to [mainX, mainRight),
// pinned-right fixed from pinnedRightX.
type aggGeometry struct {
	widths                []float32
	padding               float32
	plCount, mainCount    int
	mainX, mainRight      float32
	pinnedRightX, offsetX float32
	hasPinnedRight        bool
}

func newAggGeometry(grid GridAccess, size geo.Size) aggGeometry {
	widths := grid.HeaderColumnWidths()
	pl, pr := grid.PinnedLeftCount(), grid.PinnedRightCount()
	mainX, prX, prW := grid.HeaderRegions()
	g := aggGeometry{
		widths:       widths,
		padding:      grid.CellPadding(),
		plCount:      pl,
		mainCount:    max(len(widths)-pl-pr, 0),
		mainX:        mainX,
		mainRight:    size.Width,
		pinnedRightX: prX,
		offsetX:      grid.ScrollOffsetX(),
	}
	if pr > 0 && prW > 0 {
		g.mainRight = prX
		g.hasPinnedRight = true
	}
	if g.mainRight < g.mainX {
		g.mainRight = g.mainX
	}
	return g
}

// column returns where absolute display column col is drawn: x (clip-local for
// a main-band column, overlay-local otherwise), its width, whether it is in the
// main band, and false for a column out of range.
func (g aggGeometry) column(col int) (x, w float32, inMain, ok bool) {
	if col < 0 || col >= len(g.widths) {
		return 0, 0, false, false
	}
	start, base := 0, float32(0)
	switch {
	case col < g.plCount:
	case col < g.plCount+g.mainCount:
		start, base, inMain = g.plCount, -g.offsetX, true
	default:
		if !g.hasPinnedRight {
			return 0, 0, false, false
		}
		start, base = g.plCount+g.mainCount, g.pinnedRightX
	}
	x = base
	for i := start; i < col; i++ {
		x += g.widths[i] + g.padding
	}
	return x, g.widths[col], inMain, true
}

// drawnExtent returns where an aggregate placed by placeAggText shows its text,
// in overlay coordinates: the text is right-aligned in its box, and a
// main-band aggregate is clip-local and shows only inside [mainX, mainRight).
// shown is false when none of it is visible.
func (g aggGeometry) drawnExtent(t *canvas.Text, inMain bool) (left, right float32, shown bool) {
	right = t.Position().X + t.Size().Width
	textW := bellui.MeasureText(t.Text, t.TextSize, t.TextStyle).Width
	if textW > t.Size().Width {
		textW = t.Size().Width
	}
	left = right - textW
	if inMain {
		left, right = left+g.mainX, right+g.mainX
		if left < g.mainX {
			left = g.mainX
		}
		if right > g.mainRight {
			right = g.mainRight
		}
	}
	return left, right, right > left
}

func newAggText() *canvas.Text {
	t := canvas.NewText("", nil)
	t.TextStyle = bellui.TextStyle{Bold: true}
	t.TextSize = 13
	t.Alignment = bellui.TextAlignTrailing
	return t
}

// placeAggText draws an aggregate right-aligned inside its column, inset like
// the group label, truncated to the column width.
func placeAggText(t *canvas.Text, text string, x, y, w, h float32, tc ThemeColors) {
	const inset = 4
	inner := w - 2*inset
	if inner < 0 {
		inner = 0
	}
	t.Text = cell.TruncateText(text, inner, t.TextSize, t.TextStyle)
	t.Color = tc.GroupRowFg
	t.Move(geo.NewPos(x+inset, y+2))
	t.Resize(geo.NewSize(inner, h-4))
}

// groupRowWidget is the visual content of one group-header row: a full-width
// background, the disclosure glyph and the "key (count)" label, drawn once at
// the grid level by the GroupRowOverlay.
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

// groupLabelX is where a group row's "key (count)" label starts, from the
// row's left edge, at nesting depth.
func groupLabelX(depth int) float32 { return float32(depth)*20 + 20 }

// update lays the row out. The label is truncated with an ellipsis to end a
// small gap before labelEnd, the left edge of the first aggregate it would
// otherwise run into (#748); with no room at all it is left out.
func (g *groupRowWidget) update(key string, childCount int, expanded bool, depth int, pos geo.Position, size geo.Size, labelEnd float32, tc ThemeColors) {
	const gap = 8
	indent := float32(depth) * 20
	g.bg.FillColor = tc.GroupRowBg
	g.bg.Move(pos)
	g.bg.Resize(size)
	g.icon.Text = disclosureGlyph(expanded)
	g.icon.Color = tc.GroupRowFg
	g.icon.Move(geo.NewPos(pos.X+indent+4, pos.Y+2))
	g.icon.Resize(geo.NewSize(14, size.Height-4))
	labelX := pos.X + groupLabelX(depth)
	text := fmt.Sprintf("%s (%d)", key, childCount)
	if room := pos.X + labelEnd - gap - labelX; labelEnd < pos.X+size.Width {
		if room > 0 {
			text = cell.TruncateText(text, room, g.label.TextSize, g.label.TextStyle)
		} else {
			text = ""
		}
	}
	g.label.Text = text
	g.label.Color = tc.GroupRowFg
	g.label.Move(geo.NewPos(labelX, pos.Y+2))
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

// set updates the node. The label is the visible "key (count)" followed by
// each aggregate's spoken form, e.g. "West (3), Salary sum 300", so a screen
// reader hears the summary a sighted user sees on the row.
func (g *groupRowA11y) set(key string, childCount int, expanded bool, aggs []GroupAggregate) {
	g.key = key
	label := fmt.Sprintf("%s (%d)", key, childCount)
	for _, a := range aggs {
		label += ", " + a.Label
	}
	g.label = label
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
