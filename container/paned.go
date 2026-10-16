package container

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// Declare conformity with CanvasObject interface
var _ core.CanvasObject = (*Paned)(nil)

// Paned is a container that arranges multiple children with draggable dividers
// between them. Unlike Split (which handles exactly 2 children), Paned supports
// an arbitrary number of panels arranged horizontally or vertically.
//
// Each panel is assigned a weight that determines its proportion of the available
// space. Weights are normalized automatically.
//
// Since: 1.0
type Paned struct {
	widget.BaseWidget
	Horizontal bool
	Panels     []core.CanvasObject
	Weights    []float64 // proportional weights for each panel (auto-normalized)
}

// NewHPaned creates a horizontally arranged paned container.
// Panels are laid out left to right with vertical dividers between them.
//
// Since: 1.0
func NewHPaned(panels ...core.CanvasObject) *Paned {
	p := &Paned{
		Horizontal: true,
		Panels:     panels,
	}
	p.initWeights()
	p.BaseWidget.ExtendBaseWidget(p)
	return p
}

// NewVPaned creates a vertically arranged paned container.
// Panels are laid out top to bottom with horizontal dividers between them.
//
// Since: 1.0
func NewVPaned(panels ...core.CanvasObject) *Paned {
	p := &Paned{
		Horizontal: false,
		Panels:     panels,
	}
	p.initWeights()
	p.BaseWidget.ExtendBaseWidget(p)
	return p
}

// SetWeight sets the weight for the panel at the given index.
func (p *Paned) SetWeight(index int, weight float64) {
	if index < 0 || index >= len(p.Weights) {
		return
	}
	if weight < 0 {
		weight = 0
	}
	p.Weights[index] = weight
	p.Refresh()
}

// AddPanel appends a new panel with default weight 1.0.
func (p *Paned) AddPanel(panel core.CanvasObject) {
	p.Panels = append(p.Panels, panel)
	p.Weights = append(p.Weights, 1.0)
	p.Refresh()
}

// RemovePanel removes the panel at the given index.
func (p *Paned) RemovePanel(index int) {
	if index < 0 || index >= len(p.Panels) {
		return
	}
	p.Panels = append(p.Panels[:index], p.Panels[index+1:]...)
	p.Weights = append(p.Weights[:index], p.Weights[index+1:]...)
	p.Refresh()
}

// CreateRenderer creates the renderer for the paned container.
func (p *Paned) CreateRenderer() core.WidgetRenderer {
	p.BaseWidget.ExtendBaseWidget(p)
	p.initWeights()

	dividers := make([]*paneDivider, 0, len(p.Panels)-1)
	for i := 0; i < len(p.Panels)-1; i++ {
		dividers = append(dividers, newPaneDivider(p, i))
	}

	return &panedRenderer{
		paned:    p,
		dividers: dividers,
	}
}

func (p *Paned) initWeights() {
	if len(p.Weights) >= len(p.Panels) {
		return
	}
	p.Weights = make([]float64, len(p.Panels))
	for i := range p.Weights {
		p.Weights[i] = 1.0
	}
}

func (p *Paned) totalWeight() float64 {
	total := 0.0
	for _, w := range p.Weights {
		total += w
	}
	if total <= 0 {
		return 1.0
	}
	return total
}

// --- panedRenderer ---

type panedRenderer struct {
	paned    *Paned
	dividers []*paneDivider
}

// Destroy has nothing to release: the panel content and the divider widgets are
// CanvasObjects torn down by the framework's destroy walk. Drag state is plain
// memory and no timer, goroutine or animation is owned here.
func (r *panedRenderer) Destroy() {}

func (r *panedRenderer) Layout(size geo.Size) {
	p := r.paned
	n := len(p.Panels)
	if n == 0 {
		return
	}

	r.syncDividers()

	divCount := len(r.dividers)
	divThick := paneDividerThickness(r.paned)
	totalDivider := divThick * float32(divCount)

	var available float32
	if p.Horizontal {
		available = size.Width - totalDivider
	} else {
		available = size.Height - totalDivider
	}
	if available < 0 {
		available = 0
	}

	// Compute sizes for each panel, respecting min sizes
	panelLengths := r.computePanelLengths(available)
	totalWeight := p.totalWeight()

	offset := float32(0)
	for i, panel := range p.Panels {
		length := panelLengths[i]
		if p.Horizontal {
			panel.Move(geo.NewPos(offset, 0))
			panel.Resize(geo.NewSize(length, size.Height))
		} else {
			panel.Move(geo.NewPos(0, offset))
			panel.Resize(geo.NewSize(size.Width, length))
		}
		offset += length

		if i < divCount {
			div := r.dividers[i]
			if p.Horizontal {
				div.Move(geo.NewPos(offset, 0))
				div.Resize(geo.NewSize(divThick, size.Height))
			} else {
				div.Move(geo.NewPos(0, offset))
				div.Resize(geo.NewSize(size.Width, divThick))
			}
			offset += divThick
		}
	}

	// Update weights from actual lengths (so dragging is persistent)
	if available > 0 {
		for i, length := range panelLengths {
			p.Weights[i] = float64(length / available * float32(totalWeight))
		}
	}
}

func (r *panedRenderer) computePanelLengths(available float32) []float32 {
	p := r.paned
	n := len(p.Panels)
	lengths := make([]float32, n)
	totalWeight := p.totalWeight()

	// First pass: allocate by weight
	for i := range lengths {
		lengths[i] = float32(p.Weights[i]/totalWeight) * available
	}

	// Second pass: enforce min sizes
	for i, panel := range p.Panels {
		var minLen float32
		if p.Horizontal {
			minLen = panel.MinSize().Width
		} else {
			minLen = panel.MinSize().Height
		}
		if lengths[i] < minLen {
			lengths[i] = minLen
		}
	}

	return lengths
}

func (r *panedRenderer) MinSize() geo.Size {
	p := r.paned
	s := geo.NewSize(0, 0)
	divThick := paneDividerThickness(p)

	for i, panel := range p.Panels {
		min := panel.MinSize()
		if p.Horizontal {
			s.Width += min.Width
			s.Height = geo.Max(s.Height, min.Height)
		} else {
			s.Width = geo.Max(s.Width, min.Width)
			s.Height += min.Height
		}
		if i < len(p.Panels)-1 {
			if p.Horizontal {
				s.Width += divThick
			} else {
				s.Height += divThick
			}
		}
	}
	return s
}

func (r *panedRenderer) Objects() []core.CanvasObject {
	objects := make([]core.CanvasObject, 0, len(r.paned.Panels)+len(r.dividers))
	for i, panel := range r.paned.Panels {
		objects = append(objects, panel)
		if i < len(r.dividers) {
			objects = append(objects, r.dividers[i])
		}
	}
	return objects
}

func (r *panedRenderer) Refresh() {
	r.syncDividers()
	r.Layout(r.paned.Size())
	for _, panel := range r.paned.Panels {
		panel.Refresh()
	}
	for _, d := range r.dividers {
		d.Refresh()
	}
}

func (r *panedRenderer) syncDividers() {
	needed := len(r.paned.Panels) - 1
	if needed < 0 {
		needed = 0
	}
	for len(r.dividers) < needed {
		r.dividers = append(r.dividers, newPaneDivider(r.paned, len(r.dividers)))
	}
	// Hide removed dividers so they don't linger on the canvas.
	for i := needed; i < len(r.dividers); i++ {
		r.dividers[i].Hide()
	}
	r.dividers = r.dividers[:needed]
	for i := range r.dividers {
		r.dividers[i].index = i
		r.dividers[i].Show()
	}
}

// --- paneDivider: draggable divider between panes ---

var (
	_ core.CanvasObject  = (*paneDivider)(nil)
	_ event.Draggable    = (*paneDivider)(nil)
	_ desktop.Cursorable = (*paneDivider)(nil)
	_ event.Hoverable    = (*paneDivider)(nil)
)

type paneDivider struct {
	widget.BaseWidget
	paned          *Paned
	index          int // divider between Panels[index] and Panels[index+1]
	hovered        bool
	startDragOff   *geo.Position
	currentDragPos geo.Position
}

func newPaneDivider(p *Paned, index int) *paneDivider {
	d := &paneDivider{paned: p, index: index}
	d.ExtendBaseWidget(d)
	return d
}

func (d *paneDivider) CreateRenderer() core.WidgetRenderer {
	d.ExtendBaseWidget(d)
	th := d.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	bg := canvas.NewRectangle(th.Color(theme.ColorNameShadow, v))
	fg := canvas.NewRectangle(th.Color(theme.ColorNameForeground, v))
	return &paneDividerRenderer{
		divider:    d,
		background: bg,
		foreground: fg,
		objects:    []core.CanvasObject{bg, fg},
	}
}

func (d *paneDivider) Cursor() desktop.Cursor {
	if d.paned.Horizontal {
		return desktop.HResizeCursor
	}
	return desktop.VResizeCursor
}

func (d *paneDivider) Dragged(e *event.DragEvent) {
	if d.startDragOff == nil {
		d.currentDragPos = d.Position().Add(e.Position)
		start := e.Position.Subtract(e.Dragged)
		d.startDragOff = &start
	} else {
		d.currentDragPos = d.currentDragPos.Add(e.Dragged)
	}

	p := d.paned
	totalWeight := p.totalWeight()
	divThick := paneDividerThickness(p)
	totalDivider := divThick * float32(len(p.Panels)-1)

	var totalLen, drag float32
	if p.Horizontal {
		totalLen = p.Size().Width - totalDivider
		drag = e.Dragged.DX
	} else {
		totalLen = p.Size().Height - totalDivider
		drag = e.Dragged.DY
	}
	if totalLen <= 0 {
		return
	}

	// Convert drag pixels to weight delta
	weightDelta := float64(drag/totalLen) * totalWeight

	left := d.index
	right := d.index + 1
	if right >= len(p.Weights) {
		return
	}

	// Apply delta, clamping by min sizes
	newLeft := p.Weights[left] + weightDelta
	newRight := p.Weights[right] - weightDelta

	// Enforce minimum weight based on min sizes
	var minLeftLen, minRightLen float32
	if p.Horizontal {
		minLeftLen = p.Panels[left].MinSize().Width
		minRightLen = p.Panels[right].MinSize().Width
	} else {
		minLeftLen = p.Panels[left].MinSize().Height
		minRightLen = p.Panels[right].MinSize().Height
	}
	minLeftWeight := float64(minLeftLen/totalLen) * totalWeight
	minRightWeight := float64(minRightLen/totalLen) * totalWeight

	if newLeft < minLeftWeight {
		newLeft = minLeftWeight
		newRight = p.Weights[left] + p.Weights[right] - newLeft
	}
	if newRight < minRightWeight {
		newRight = minRightWeight
		newLeft = p.Weights[left] + p.Weights[right] - newRight
	}

	p.Weights[left] = newLeft
	p.Weights[right] = newRight
	p.Refresh()
}

func (d *paneDivider) DragEnd() {
	d.startDragOff = nil
}

func (d *paneDivider) HoverIn(_ *event.HoverEvent) {
	d.hovered = true
	d.Refresh()
}

func (d *paneDivider) HoverMoved(_ *event.HoverEvent) {}

func (d *paneDivider) HoverOut() {
	d.hovered = false
	d.Refresh()
}

// --- paneDividerRenderer ---

type paneDividerRenderer struct {
	divider    *paneDivider
	background *canvas.Rectangle
	foreground *canvas.Rectangle
	objects    []core.CanvasObject
}

// Destroy has nothing to release: the renderer holds only canvas.Rectangle
// handles, torn down by the framework's destroy walk.
func (r *paneDividerRenderer) Destroy() {}

func (r *paneDividerRenderer) Layout(size geo.Size) {
	r.background.Resize(size)
	thick := paneDividerHandleThickness(r.divider.paned)
	length := paneDividerHandleLength(r.divider.paned)

	var x, y, w, h float32
	if r.divider.paned.Horizontal {
		x = (size.Width - thick) / 2
		y = (size.Height - length) / 2
		w = thick
		h = length
	} else {
		x = (size.Width - length) / 2
		y = (size.Height - thick) / 2
		w = length
		h = thick
	}
	r.foreground.Move(geo.NewPos(x, y))
	r.foreground.Resize(geo.NewSize(w, h))
}

func (r *paneDividerRenderer) MinSize() geo.Size {
	thick := paneDividerThickness(r.divider.paned)
	length := paneDividerLength(r.divider.paned)
	if r.divider.paned.Horizontal {
		return geo.NewSize(thick, length)
	}
	return geo.NewSize(length, thick)
}

func (r *paneDividerRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *paneDividerRenderer) Refresh() {
	th := r.divider.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	if r.divider.hovered {
		r.background.FillColor = th.Color(theme.ColorNameHover, v)
	} else {
		r.background.FillColor = th.Color(theme.ColorNameShadow, v)
	}
	r.background.Refresh()
	r.foreground.FillColor = th.Color(theme.ColorNameForeground, v)
	r.foreground.Refresh()
	r.Layout(r.divider.Size())
}

// Theme-based sizing helpers — mirror the Split divider sizing convention.

func paneDividerThickness(p *Paned) float32 {
	return p.Theme().Size(theme.SizeNamePadding) * 2
}

func paneDividerLength(p *Paned) float32 {
	return p.Theme().Size(theme.SizeNamePadding) * 6
}

func paneDividerHandleThickness(p *Paned) float32 {
	return p.Theme().Size(theme.SizeNamePadding) / 2
}

func paneDividerHandleLength(p *Paned) float32 {
	return p.Theme().Size(theme.SizeNamePadding) * 4
}
