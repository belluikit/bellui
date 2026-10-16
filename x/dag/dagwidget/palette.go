package dagwidget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/dag/dag"
)

// Palette displays registered components grouped by category.
// Users can click a component to add it to the DAG canvas.
type Palette struct {
	widget.BaseWidget

	Registry  *dag.Registry
	DAGCanvas *DAGCanvas

	content *bellui.Container
}

// NewPalette creates a new component palette.
func NewPalette(registry *dag.Registry, dc *DAGCanvas) *Palette {
	p := &Palette{
		Registry:  registry,
		DAGCanvas: dc,
	}
	p.rebuild()
	p.ExtendBaseWidget(p)
	return p
}

// Rebuild reconstructs the palette UI from the current registry.
func (p *Palette) Rebuild() {
	p.rebuild()
	p.Refresh()
}

func (p *Palette) rebuild() {
	var items []core.CanvasObject

	categories := p.Registry.Categories()
	for _, cat := range categories {
		// Category header
		header := widget.NewRichTextFromMarkdown("**" + cat + "**")
		items = append(items, header)

		comps := p.Registry.ByCategory(cat)
		for _, comp := range comps {
			item := newPaletteItem(comp, p)
			items = append(items, item)
		}

		// Spacer between categories
		items = append(items, widget.NewSeparator())
	}

	p.content = container.NewVBox(items...)
}

func (p *Palette) CreateRenderer() core.WidgetRenderer {
	scroll := container.NewScroll(p.content)
	scroll.SetMinSize(geo.NewSize(240, 0))
	return widget.NewSimpleRenderer(scroll)
}

// paletteItem is a single component entry in the palette.
// It supports both click (adds at center) and drag-and-drop (adds at drop position).
type paletteItem struct {
	widget.BaseWidget

	component  *dag.Component
	palette    *Palette
	lastAbsPos geo.Position // updated during drag
	dragging   bool
}

func newPaletteItem(comp *dag.Component, p *Palette) *paletteItem {
	item := &paletteItem{
		component: comp,
		palette:   p,
	}
	item.ExtendBaseWidget(item)
	return item
}

var (
	_ event.Tappable         = (*paletteItem)(nil)
	_ event.Draggable        = (*paletteItem)(nil)
	_ desktop.Cursorable     = (*paletteItem)(nil)
	_ core.DragImageProvider = (*paletteItem)(nil)
)

// Cursor shows an open hand over palette entries to hint they can be dragged
// onto the canvas.
func (item *paletteItem) Cursor() desktop.Cursor {
	return desktop.GrabCursor
}

// DragImage returns the ghost card painted under the pointer while dragging
// a component out of the palette — without it there is no visible sign that
// the drag is in progress until the drop. The driver also switches to the
// grabbing cursor for DragImageProvider drags.
func (item *paletteItem) DragImage() core.CanvasObject {
	th := item.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	bg := &canvas.Rectangle{
		FillColor:    withAlpha(th.Color(theme.ColorNameButton, v), 220),
		StrokeColor:  th.Color(theme.ColorNamePrimary, v),
		StrokeWidth:  1.5,
		CornerRadius: 8,
	}
	label := widget.NewLabel(item.component.Name)
	return container.NewStack(bg, container.NewPadded(label))
}

// Tapped adds this component at the viewport center (fallback when not dragged).
func (item *paletteItem) Tapped(_ *event.PointEvent) {
	dc := item.palette.DAGCanvas
	if dc == nil || !dc.Capabilities.AddNodes {
		return
	}
	size := dc.Size()
	centerScreen := geo.NewPos(size.Width/2, size.Height/2)
	graphPos := dc.screenToGraph(centerScreen)
	dc.AddComponentNode(item.component, [2]float32{graphPos.X, graphPos.Y})
}

// Dragged tracks the cursor position during a drag from the palette.
func (item *paletteItem) Dragged(ev *event.DragEvent) {
	item.dragging = true
	item.lastAbsPos = ev.AbsolutePosition
}

// DragEnd creates a node at the drop position if released over the DAG canvas.
func (item *paletteItem) DragEnd() {
	if !item.dragging {
		return
	}
	item.dragging = false

	dc := item.palette.DAGCanvas
	if dc == nil || !dc.Capabilities.AddNodes {
		return
	}

	// Check if the drop position is within the DAGCanvas bounds
	driver := bellui.CurrentApp().Driver()
	canvasAbs := driver.AbsolutePositionForObject(dc)
	canvasSize := dc.Size()

	rel := geo.NewPos(item.lastAbsPos.X-canvasAbs.X, item.lastAbsPos.Y-canvasAbs.Y)
	if rel.X < 0 || rel.Y < 0 || rel.X > canvasSize.Width || rel.Y > canvasSize.Height {
		return // dropped outside canvas
	}

	graphPos := dc.screenToGraph(rel)
	dc.AddComponentNode(item.component, [2]float32{graphPos.X, graphPos.Y})
}

func (item *paletteItem) CreateRenderer() core.WidgetRenderer {
	bg := &canvas.Rectangle{
		CornerRadius: 4,
	}
	name := canvas.NewText(item.component.Name, nil)
	name.TextSize = 13

	// Stream type summary line
	summary := streamSummary(item.component)
	desc := canvas.NewText(summary, nil)
	desc.TextSize = 11

	r := &paletteItemRenderer{
		item: item,
		bg:   bg,
		name: name,
		desc: desc,
	}
	r.applyTheme()
	return r
}

type paletteItemRenderer struct {
	item *paletteItem
	bg   *canvas.Rectangle
	name *canvas.Text
	desc *canvas.Text
}

const (
	paletteItemPad = 6 // inner padding of a palette entry
	paletteItemGap = 2 // space between the name and the port summary
)

// Layout stacks the name and the port summary at their measured heights. They
// were fixed 16 and 14 px boxes, but the summary's arrow comes from a fallback
// font whose line is far taller than 14 px at 11 pt, so it was clipped.
func (r *paletteItemRenderer) Layout(size geo.Size) {
	r.bg.Resize(size)
	r.bg.Move(geo.NewPos(0, 0))
	pad := float32(paletteItemPad)
	nameH, descH := r.name.MinSize().Height, r.desc.MinSize().Height
	r.name.Move(geo.NewPos(pad, pad))
	r.name.Resize(geo.NewSize(size.Width-pad*2, nameH))
	r.desc.Move(geo.NewPos(pad, pad+nameH+paletteItemGap))
	r.desc.Resize(geo.NewSize(size.Width-pad*2, descH))
}

func (r *paletteItemRenderer) MinSize() geo.Size {
	h := 2*paletteItemPad + r.name.MinSize().Height + paletteItemGap + r.desc.MinSize().Height
	w := max32(160, 2*paletteItemPad+max32(r.name.MinSize().Width, r.desc.MinSize().Width))
	return geo.NewSize(w, h)
}

func (r *paletteItemRenderer) Refresh() {
	r.name.Text = r.item.component.Name
	r.applyTheme()
	r.name.Refresh()
	r.desc.Refresh()
	r.bg.Refresh()
}

func (r *paletteItemRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.bg, r.name, r.desc}
}

func (r *paletteItemRenderer) Destroy() {}

func (r *paletteItemRenderer) applyTheme() {
	th := r.item.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	r.bg.FillColor = th.Color(theme.ColorNameButton, v)
	r.name.Color = th.Color(theme.ColorNameForeground, v)
	r.desc.Color = th.Color(theme.ColorNamePlaceHolder, v)
}

// streamSummary returns a short string describing a component's port types.
func streamSummary(c *dag.Component) string {
	var in, out string
	if len(c.Inputs) > 0 {
		in = c.Inputs[0].Stream.Name
		if len(c.Inputs) > 1 {
			in += "..."
		}
	}
	if len(c.Outputs) > 0 {
		out = c.Outputs[0].Stream.Name
		if len(c.Outputs) > 1 {
			out += "..."
		}
	}
	switch {
	case in != "" && out != "":
		return in + " → " + out
	case out != "":
		return "→ " + out
	case in != "":
		return in + " →"
	default:
		return ""
	}
}
