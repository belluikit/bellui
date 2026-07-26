package dagwidget

import (
	"fmt"
	"image/color"
	"strconv"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// fyneDriver is a tiny indirection to keep widget code terse.
func fyneDriver() bellui.Driver {
	return bellui.CurrentApp().Driver()
}

// parseHexColor parses "#rrggbb" into a color. Reports ok=false for any
// other format.
func parseHexColor(s string) (color.NRGBA, bool) {
	if len(s) != 7 || s[0] != '#' {
		return color.NRGBA{}, false
	}
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return color.NRGBA{}, false
	}
	return color.NRGBA{
		R: uint8(v >> 16),
		G: uint8(v >> 8),
		B: uint8(v),
		A: 0xff,
	}, true
}

// --- GroupWidget renderer ---

type groupRenderer struct {
	group *GroupWidget
	body  *canvas.Rectangle
}

func newGroupRenderer(g *GroupWidget) *groupRenderer {
	r := &groupRenderer{
		group: g,
		body:  &canvas.Rectangle{},
	}
	r.applyTheme()
	return r
}

func (r *groupRenderer) Layout(size geo.Size) {
	g := r.group
	z := g.zoom()

	r.body.Move(geo.NewPos(0, 0))
	r.body.Resize(size)
	r.body.CornerRadius = 10 * z

	titleH := groupTitleHeight * z
	if g.collapsed() {
		// The title bar covers the whole card so the entire collapsed group
		// is draggable/tappable.
		titleH = size.Height
	}
	g.titleBar.Move(geo.NewPos(0, 0))
	g.titleBar.Resize(geo.NewSize(size.Width, titleH))

	// Collapse toggle at the right end of the title area, connector handle
	// just left of it. Both stay vertically centered in the title strip.
	stripH := groupTitleHeight * z
	if g.collapsed() {
		stripH = size.Height
	}
	cb := collapseButtonSize * z
	ch := connectorHandleSize * z
	pad := 4 * z
	g.collapse.Move(geo.NewPos(size.Width-cb-pad, (stripH-cb)/2))
	g.collapse.Resize(geo.NewSquareSize(cb))
	g.connector.Move(geo.NewPos(size.Width-cb-ch-pad*2, (stripH-ch)/2))
	g.connector.Resize(geo.NewSquareSize(ch))

	// Resize handles centered on the border corners/midpoints.
	hs := handleSize * z
	for _, hw := range g.handles {
		fx := float32(hw.dirX+1) / 2
		fy := float32(hw.dirY+1) / 2
		hw.Move(geo.NewPos(fx*size.Width-hs/2, fy*size.Height-hs/2))
		hw.Resize(geo.NewSquareSize(hs))
	}
}

func (r *groupRenderer) MinSize() geo.Size {
	return geo.NewSize(0, 0)
}

func (r *groupRenderer) Refresh() {
	r.applyTheme()
	r.body.Refresh()
	r.group.titleBar.Refresh()
	r.group.collapse.Refresh()
	c := r.group.dagCanvas
	if c == nil || c.Capabilities.GroupEdges {
		r.group.connector.Show()
	} else {
		r.group.connector.Hide()
	}
	r.group.connector.Refresh()
	setHandlesVisible(r.group.handles, r.group.handlesVisible())
	for _, hw := range r.group.handles {
		hw.Refresh()
	}
}

func (r *groupRenderer) Objects() []core.CanvasObject {
	g := r.group
	objs := []core.CanvasObject{r.body, g.titleBar, g.connector, g.collapse}
	for _, hw := range g.handles {
		objs = append(objs, hw)
	}
	return objs
}

func (r *groupRenderer) Destroy() {}

func (r *groupRenderer) applyTheme() {
	g := r.group
	th := g.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	z := g.zoom()

	tint := th.Color(theme.ColorNamePrimary, v)
	if grp := g.group(); grp != nil {
		if c, ok := parseHexColor(grp.Color); ok {
			tint = c
		}
	}

	fillAlpha := uint8(28)
	stroke := th.Color(theme.ColorNameSeparator, v)
	strokeW := 1.5 * z
	switch {
	case g.DropHighlight:
		fillAlpha = 48
		stroke = th.Color(theme.ColorNamePrimary, v)
		strokeW = 2.5 * z
	case g.Selected:
		stroke = th.Color(theme.ColorNamePrimary, v)
		strokeW = 2.5 * z
	}
	r.body.FillColor = withAlpha(tint, fillAlpha)
	r.body.StrokeColor = stroke
	r.body.StrokeWidth = strokeW
}

// --- title bar renderer ---

type groupTitleBarRenderer struct {
	bar   *groupTitleBar
	bg    *canvas.Rectangle
	label *canvas.Text
	count *canvas.Text // "N nodes", collapsed card only
}

func newGroupTitleBarRenderer(t *groupTitleBar) *groupTitleBarRenderer {
	r := &groupTitleBarRenderer{
		bar:   t,
		bg:    &canvas.Rectangle{},
		label: &canvas.Text{TextSize: 13},
		count: &canvas.Text{TextSize: 11},
	}
	r.label.TextStyle.Bold = true
	r.applyTheme()
	return r
}

func (r *groupTitleBarRenderer) Layout(size geo.Size) {
	t := r.bar
	z := t.gw.zoom()
	pad := 8 * z

	r.bg.Move(geo.NewPos(0, 0))
	r.bg.Resize(size)
	r.bg.CornerRadius = 10 * z

	// Leave room for the connector + collapse controls on the right.
	controls := (collapseButtonSize + connectorHandleSize + 12) * z
	r.label.TextSize = 13 * z
	r.label.Move(geo.NewPos(pad, 3*z))
	r.label.Resize(geo.NewSize(size.Width-pad-controls, groupTitleHeight*z-6*z))

	r.count.TextSize = 11 * z
	r.count.Move(geo.NewPos(pad, groupTitleHeight*z))
	r.count.Resize(geo.NewSize(size.Width-pad-controls, size.Height-groupTitleHeight*z))
}

func (r *groupTitleBarRenderer) MinSize() geo.Size {
	return geo.NewSize(0, 0)
}

func (r *groupTitleBarRenderer) Refresh() {
	r.applyTheme()
	r.bg.Refresh()
	r.label.Refresh()
	r.count.Refresh()
}

func (r *groupTitleBarRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.bg, r.label, r.count}
}

func (r *groupTitleBarRenderer) Destroy() {}

func (r *groupTitleBarRenderer) applyTheme() {
	t := r.bar
	th := t.gw.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	tint := th.Color(theme.ColorNamePrimary, v)
	label := "Group"
	count := ""
	if grp := t.gw.group(); grp != nil {
		if c, ok := parseHexColor(grp.Color); ok {
			tint = c
		}
		if grp.Label != "" {
			label = grp.Label
		}
		if grp.Collapsed {
			count = fmt.Sprintf("%d nodes", len(grp.Members))
		}
	}
	r.bg.FillColor = withAlpha(tint, 52)
	r.label.Text = label
	r.label.Color = th.Color(theme.ColorNameForeground, v)
	r.count.Text = count
	r.count.Color = withAlpha(th.Color(theme.ColorNameForeground, v), 170)
}

// --- collapse button renderer ---

type groupCollapseRenderer struct {
	btn  *groupCollapseButton
	icon *canvas.Image
}

func newGroupCollapseRenderer(b *groupCollapseButton) *groupCollapseRenderer {
	r := &groupCollapseRenderer{
		btn:  b,
		icon: canvas.NewImageFromResource(theme.MenuDropUpIcon()),
	}
	r.icon.FillMode = canvas.ImageFillContain
	r.Refresh()
	return r
}

func (r *groupCollapseRenderer) Layout(size geo.Size) {
	r.icon.Move(geo.NewPos(0, 0))
	r.icon.Resize(size)
}

func (r *groupCollapseRenderer) MinSize() geo.Size {
	return geo.NewSquareSize(collapseButtonSize)
}

func (r *groupCollapseRenderer) Refresh() {
	if r.btn.gw.collapsed() {
		r.icon.Resource = theme.MenuDropDownIcon()
	} else {
		r.icon.Resource = theme.MenuDropUpIcon()
	}
	r.icon.Refresh()
}

func (r *groupCollapseRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.icon}
}

func (r *groupCollapseRenderer) Destroy() {}

// --- connector handle renderer ---

type groupConnectorRenderer struct {
	handle *groupConnectorHandle
	circle *canvas.Circle
}

func newGroupConnectorRenderer(h *groupConnectorHandle) *groupConnectorRenderer {
	r := &groupConnectorRenderer{
		handle: h,
		circle: &canvas.Circle{},
	}
	r.Refresh()
	return r
}

func (r *groupConnectorRenderer) Layout(size geo.Size) {
	r.circle.Move(geo.NewPos(0, 0))
	r.circle.Resize(size)
}

func (r *groupConnectorRenderer) MinSize() geo.Size {
	return geo.NewSquareSize(connectorHandleSize)
}

func (r *groupConnectorRenderer) Refresh() {
	h := r.handle
	th := h.gw.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	z := h.gw.zoom()
	r.circle.FillColor = withAlpha(th.Color(theme.ColorNamePrimary, v), 60)
	r.circle.StrokeColor = th.Color(theme.ColorNamePrimary, v)
	r.circle.StrokeWidth = 1.5 * z
	r.circle.Refresh()
}

func (r *groupConnectorRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.circle}
}

func (r *groupConnectorRenderer) Destroy() {}
