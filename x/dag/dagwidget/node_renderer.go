package dagwidget

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

type nodeRenderer struct {
	node      *NodeWidget
	bg        *canvas.Rectangle
	titleText *canvas.Text

	// Styler-driven parts, hidden while the current NodeStyle leaves them
	// zero. style caches the last computed NodeStyle for Layout.
	icon     *canvas.Image
	subtitle *canvas.Text
	accent   *canvas.Rectangle
	style    NodeStyle
}

func newNodeRenderer(n *NodeWidget) *nodeRenderer {
	bg := &canvas.Rectangle{
		FillColor:    color.Transparent,
		StrokeWidth:  1,
		CornerRadius: 8,
	}
	titleText := &canvas.Text{
		Text:     n.Label,
		TextSize: 14,
	}
	icon := &canvas.Image{FillMode: canvas.ImageFillContain}
	icon.Hide()
	subtitle := &canvas.Text{TextSize: 10}
	subtitle.Hide()
	accent := &canvas.Rectangle{}
	accent.Hide()
	r := &nodeRenderer{
		node:      n,
		bg:        bg,
		titleText: titleText,
		icon:      icon,
		subtitle:  subtitle,
		accent:    accent,
	}
	// Sync the style now so a freshly built widget renders styled on its
	// first paint (pre-rebrand tracker #36); the framework's initial Layout places the parts.
	r.syncStyle()
	r.applyTheme()
	return r
}

func (r *nodeRenderer) Layout(size geo.Size) {
	z := r.node.zoom()

	r.bg.Move(geo.NewPos(0, 0))
	r.bg.Resize(size)
	r.bg.CornerRadius = 8 * z

	// Scaled dimensions
	pad := nodePadding * z
	ps := portSize * z
	th := titleHeight * z

	r.titleText.TextSize = 14 * z
	titleX := ps + pad
	if r.style.Icon != nil {
		is := nodeIconSize * z
		r.icon.Move(geo.NewPos(titleX, pad+(th-is)/2))
		r.icon.Resize(geo.NewSquareSize(is))
		titleX += is + pad/2
	}
	// Connector handle at the right end of the title strip, clear of the
	// output ports on the right border; the title text stops short of it.
	ch := connectorHandleSize * z
	r.node.connector.Move(geo.NewPos(size.Width-ch-ps-pad/2, pad+(th-ch)/2))
	r.node.connector.Resize(geo.NewSquareSize(ch))
	r.titleText.Move(geo.NewPos(titleX, pad))
	r.titleText.Resize(geo.NewSize(size.Width-titleX-ps-pad-ch, th))

	if r.style.Subtitle != "" {
		r.subtitle.TextSize = nodeSubtitleSize * z
		r.subtitle.Move(geo.NewPos(ps+pad, pad+th))
		r.subtitle.Resize(geo.NewSize(size.Width-2*(ps+pad), nodeSubtitleSize*1.4*z))
	}
	if r.style.Accent != nil {
		bar := nodeAccentWidth * z
		r.accent.CornerRadius = bar / 2
		r.accent.Move(geo.NewPos(0, pad))
		r.accent.Resize(geo.NewSize(bar, size.Height-2*pad))
	}

	// Input ports along left edge
	n := r.node
	if len(n.Inputs) > 0 {
		startY := th + pad*2
		spacing := (size.Height - startY - pad) / float32(len(n.Inputs))
		for i, pw := range n.Inputs {
			y := startY + spacing*float32(i) + (spacing-ps)/2
			pw.Move(geo.NewPos(-ps/2, y))
			pw.Resize(geo.NewSquareSize(ps))
		}
	}

	// Output ports along right edge
	if len(n.Outputs) > 0 {
		startY := th + pad*2
		spacing := (size.Height - startY - pad) / float32(len(n.Outputs))
		for i, pw := range n.Outputs {
			y := startY + spacing*float32(i) + (spacing-ps)/2
			pw.Move(geo.NewPos(size.Width-ps/2, y))
			pw.Resize(geo.NewSquareSize(ps))
		}
	}

	// Resize handles centered on the border corners/midpoints.
	hs := handleSize * z
	for _, hw := range n.handles {
		fx := float32(hw.dirX+1) / 2 // -1,0,1 -> 0, 0.5, 1
		fy := float32(hw.dirY+1) / 2
		hw.Move(geo.NewPos(fx*size.Width-hs/2, fy*size.Height-hs/2))
		hw.Resize(geo.NewSquareSize(hs))
	}
}

func (r *nodeRenderer) MinSize() geo.Size {
	return r.node.MinSize()
}

// syncStyle recomputes the NodeStyle and syncs the icon, subtitle, and
// accent objects' content and visibility. Presentation only — MinSize and
// port geometry are untouched (portCenterGraph must stay a pure function of
// dag.Node). No layout or repaint: the construction path relies on the
// initial Layout/paint; the refresh path is refreshStyle.
func (r *nodeRenderer) syncStyle() {
	n := r.node
	r.style = NodeStyle{}
	if n.dagCanvas != nil && n.dagCanvas.NodeStyler != nil {
		if node := n.dagCanvas.Graph.Nodes[n.NodeID]; node != nil {
			r.style = n.dagCanvas.NodeStyler(node)
		}
	}

	if r.style.Icon != nil {
		r.icon.Resource = r.style.Icon
		r.icon.Show()
	} else {
		r.icon.Resource = nil
		r.icon.Hide()
	}
	if r.style.Subtitle != "" {
		r.subtitle.Text = r.style.Subtitle
		r.subtitle.Show()
	} else {
		r.subtitle.Hide()
	}
	if r.style.Accent != nil {
		r.accent.FillColor = r.style.Accent
		r.accent.Show()
	} else {
		r.accent.Hide()
	}
}

// refreshStyle re-syncs the style and repaints the styled parts. Style
// changes shift the title and grow/shrink the styled parts, so it re-layouts
// at the current size — a Refresh alone applies them.
func (r *nodeRenderer) refreshStyle() {
	r.syncStyle()
	r.Layout(r.node.Size())
	r.icon.Refresh()
	r.subtitle.Refresh()
	r.accent.Refresh()
}

func (r *nodeRenderer) Refresh() {
	r.titleText.Text = r.node.Label
	r.refreshStyle()
	r.applyTheme()
	r.titleText.Refresh()
	r.bg.Refresh()
	for _, pw := range r.node.Inputs {
		pw.Refresh()
	}
	for _, pw := range r.node.Outputs {
		pw.Refresh()
	}
	if r.node.dagCanvas == nil || r.node.dagCanvas.Capabilities.LinkEdges {
		r.node.connector.Show()
	} else {
		r.node.connector.Hide()
	}
	r.node.connector.Refresh()
	setHandlesVisible(r.node.handles, r.node.handlesVisible())
	for _, hw := range r.node.handles {
		hw.Refresh()
	}
}

func (r *nodeRenderer) Objects() []core.CanvasObject {
	objs := []core.CanvasObject{r.bg, r.accent, r.icon, r.subtitle, r.titleText}
	for _, pw := range r.node.Inputs {
		objs = append(objs, pw)
	}
	for _, pw := range r.node.Outputs {
		objs = append(objs, pw)
	}
	if r.node.Content != nil {
		objs = append(objs, r.node.Content)
	}
	objs = append(objs, r.node.connector)
	// Handles last so they sit on top of the border and ports.
	for _, hw := range r.node.handles {
		objs = append(objs, hw)
	}
	return objs
}

func (r *nodeRenderer) Destroy() {}

func (r *nodeRenderer) applyTheme() {
	th := r.node.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	z := r.node.zoom()
	switch {
	case r.node.DropHighlight:
		r.bg.FillColor = th.Color(theme.ColorNameButton, v)
		r.bg.StrokeColor = th.Color(theme.ColorNamePrimary, v)
		r.bg.StrokeWidth = 2.5 * z
	case r.node.Selected:
		r.bg.FillColor = th.Color(theme.ColorNameSelection, v)
		r.bg.StrokeColor = th.Color(theme.ColorNamePrimary, v)
		r.bg.StrokeWidth = 2 * z
	default:
		r.bg.FillColor = th.Color(theme.ColorNameButton, v)
		r.bg.StrokeColor = th.Color(theme.ColorNameSeparator, v)
		r.bg.StrokeWidth = 1 * z
	}
	r.titleText.Color = th.Color(theme.ColorNameForeground, v)
	r.subtitle.Color = withAlpha(th.Color(theme.ColorNameForeground, v), 170)
}

// --- connector handle renderer ---

type nodeConnectorRenderer struct {
	handle *nodeConnectorHandle
	circle *canvas.Circle
}

func newNodeConnectorRenderer(h *nodeConnectorHandle) *nodeConnectorRenderer {
	r := &nodeConnectorRenderer{
		handle: h,
		circle: &canvas.Circle{},
	}
	r.Refresh()
	return r
}

func (r *nodeConnectorRenderer) Layout(size geo.Size) {
	r.circle.Move(geo.NewPos(0, 0))
	r.circle.Resize(size)
}

func (r *nodeConnectorRenderer) MinSize() geo.Size {
	return geo.NewSquareSize(connectorHandleSize)
}

func (r *nodeConnectorRenderer) Refresh() {
	h := r.handle
	th := h.nw.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	z := h.nw.zoom()
	r.circle.FillColor = withAlpha(th.Color(theme.ColorNamePrimary, v), 60)
	r.circle.StrokeColor = th.Color(theme.ColorNamePrimary, v)
	r.circle.StrokeWidth = 1.5 * z
	r.circle.Refresh()
}

func (r *nodeConnectorRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.circle}
}

func (r *nodeConnectorRenderer) Destroy() {}
