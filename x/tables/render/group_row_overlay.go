package render

import (
	"fmt"
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
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

	var content []core.CanvasObject
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
		gw.update(key, childCount, expanded, depth, geo.NewPos(0, y), geo.NewSize(size.Width, rowH), tc)
		content = append(content, gw.objects...)

		// Accessible RoleTableRow node (semantics only, invisible), spanning
		// the full row so touch exploration finds it over any band (#111).
		ga.set(key, childCount, expanded)
		ga.Move(geo.NewPos(0, y))
		ga.Resize(geo.NewSize(size.Width, rowH))
		content = append(content, ga)
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

func (g *groupRowWidget) update(key string, childCount int, expanded bool, depth int, pos geo.Position, size geo.Size, tc ThemeColors) {
	indent := float32(depth) * 20
	g.bg.FillColor = tc.GroupRowBg
	g.bg.Move(pos)
	g.bg.Resize(size)
	g.icon.Text = disclosureGlyph(expanded)
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
