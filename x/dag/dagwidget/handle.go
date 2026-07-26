package dagwidget

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

// handleSize is the resize-handle square's side in graph units (scaled by zoom).
const handleSize float32 = 8

// resizable is a canvas element that supports border resize handles.
type resizable interface {
	core.Widget

	// resizeCanvas returns the owning DAGCanvas (nil when detached).
	resizeCanvas() *DAGCanvas
	// storedGeometry returns the element's model position and stored size in
	// graph units (a zero size means auto-size). ok is false when the model
	// object no longer exists.
	storedGeometry() (pos, size [2]float32, ok bool)
	// setStoredGeometry mutates the model position and size live during a drag.
	setStoredGeometry(pos, size [2]float32)
	// effectiveGraphSize returns the element's current on-canvas size in
	// graph units (stored size clamped to the minimum).
	effectiveGraphSize() geo.Size
	// minGraphSize returns the smallest allowed size in graph units.
	minGraphSize() geo.Size
	// pushResizeCommand records the finished resize for undo; only called
	// when the canvas has a command stack and the geometry actually changed.
	pushResizeCommand(oldPos, oldSize, newPos, newSize [2]float32)
}

// Handle direction sets per element kind. Nodes leave out the left/right
// midpoints because ports own those edges; annotations resize from corners
// only to keep the small boxes uncluttered.
var (
	nodeHandleDirs = [][2]int{
		{-1, -1}, {1, -1}, {-1, 1}, {1, 1}, // corners
		{0, -1}, {0, 1}, // top/bottom midpoints
	}
	groupHandleDirs = [][2]int{
		{-1, -1}, {1, -1}, {-1, 1}, {1, 1}, // corners
		{0, -1}, {0, 1}, {-1, 0}, {1, 0}, // edge midpoints
	}
	annotationHandleDirs = [][2]int{
		{-1, -1}, {1, -1}, {-1, 1}, {1, 1}, // corners
	}
)

// HandleWidget is a small draggable square shown on a selected element's
// border. Dragging it resizes the element in graph coordinates, clamped to
// its minimum size; handles on the left/top edges move the origin as well so
// the opposite border stays anchored.
type HandleWidget struct {
	widget.BaseWidget

	target resizable
	// dirX/dirY select which borders this handle moves: -1 left/top,
	// +1 right/bottom, 0 none.
	dirX, dirY int

	dragging  bool
	startPos  [2]float32
	startSize [2]float32
}

var (
	_ core.Widget        = (*HandleWidget)(nil)
	_ event.Draggable    = (*HandleWidget)(nil)
	_ desktop.Cursorable = (*HandleWidget)(nil)
)

// Cursor hints the resize direction: horizontal/vertical arrows on the edge
// midpoints, a crosshair on the corners (no diagonal standard cursor exists).
func (h *HandleWidget) Cursor() desktop.Cursor {
	switch {
	case h.dirY == 0:
		return desktop.HResizeCursor
	case h.dirX == 0:
		return desktop.VResizeCursor
	default:
		return desktop.CrosshairCursor
	}
}

// newHandleSet creates resize handles for an element at the given border
// positions (see the *HandleDirs sets above).
func newHandleSet(t resizable, dirs [][2]int) []*HandleWidget {
	handles := make([]*HandleWidget, 0, len(dirs))
	for _, d := range dirs {
		h := &HandleWidget{target: t, dirX: d[0], dirY: d[1]}
		h.ExtendBaseWidget(h)
		h.Hide() // shown while the element is selected
		handles = append(handles, h)
	}
	return handles
}

// setHandlesVisible shows or hides a handle set as one unit. Called from the
// owning widget's renderer Refresh and from the canvas-side affordance sync
// (dagCanvasRenderer.Layout), which is what makes a mid-session Capabilities
// change take effect without a per-widget Refresh.
func setHandlesVisible(handles []*HandleWidget, visible bool) {
	for _, hw := range handles {
		if visible {
			hw.Show()
		} else {
			hw.Hide()
		}
	}
}

func (h *HandleWidget) MinSize() geo.Size {
	return geo.NewSquareSize(handleSize)
}

func (h *HandleWidget) zoom() float32 {
	if c := h.target.resizeCanvas(); c != nil {
		return c.Zoom
	}
	return 1.0
}

// Dragged resizes the target, converting the screen delta to graph units.
func (h *HandleWidget) Dragged(ev *event.DragEvent) {
	t := h.target
	c := t.resizeCanvas()
	if c == nil || !c.Capabilities.MoveResize {
		return
	}
	pos, stored, ok := t.storedGeometry()
	if !ok {
		return
	}
	if !h.dragging {
		h.dragging = true
		h.startPos = pos
		h.startSize = stored
	}

	dx := ev.Dragged.DX / c.Zoom
	dy := ev.Dragged.DY / c.Zoom
	ms := t.minGraphSize()
	cur := t.effectiveGraphSize()

	switch h.dirX {
	case 1:
		stored[0] = max32(cur.Width+dx, ms.Width)
	case -1:
		newW := max32(cur.Width-dx, ms.Width)
		pos[0] += cur.Width - newW
		stored[0] = newW
	}
	switch h.dirY {
	case 1:
		stored[1] = max32(cur.Height+dy, ms.Height)
	case -1:
		newH := max32(cur.Height-dy, ms.Height)
		pos[1] += cur.Height - newH
		stored[1] = newH
	}

	t.setStoredGeometry(pos, stored)
	c.Refresh()
}

func (h *HandleWidget) DragEnd() {
	if !h.dragging {
		return
	}
	h.dragging = false
	c := h.target.resizeCanvas()
	if c == nil {
		return
	}
	pos, size, ok := h.target.storedGeometry()
	if !ok {
		return
	}
	if pos == h.startPos && size == h.startSize {
		return
	}
	// Recorded without executing — the element already has the final geometry.
	h.target.pushResizeCommand(h.startPos, h.startSize, pos, size)
}

func (h *HandleWidget) CreateRenderer() core.WidgetRenderer {
	r := &handleRenderer{
		handle: h,
		rect:   &canvas.Rectangle{StrokeWidth: 1},
	}
	r.applyTheme()
	return r
}

type handleRenderer struct {
	handle *HandleWidget
	rect   *canvas.Rectangle
}

func (r *handleRenderer) Layout(size geo.Size) {
	r.rect.Move(geo.NewPos(0, 0))
	r.rect.Resize(size)
}

func (r *handleRenderer) MinSize() geo.Size {
	return r.handle.MinSize()
}

func (r *handleRenderer) Refresh() {
	r.applyTheme()
	r.rect.Refresh()
}

func (r *handleRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.rect}
}

func (r *handleRenderer) Destroy() {}

func (r *handleRenderer) applyTheme() {
	th := r.handle.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	z := r.handle.zoom()
	r.rect.FillColor = th.Color(theme.ColorNamePrimary, v)
	r.rect.StrokeColor = th.Color(theme.ColorNameBackground, v)
	r.rect.StrokeWidth = 1 * z
	r.rect.CornerRadius = 2 * z
}
