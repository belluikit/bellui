package dagwidget

import (
	"image/color"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/dag/dag"
)

// Annotation geometry in graph units (scaled by zoom at render time).
const (
	annotationMinW     float32 = 80
	annotationMinH     float32 = 40
	annotationTextSize float32 = 12
	annotationPadding  float32 = 8
	defaultAnnotationW float32 = 160
	defaultAnnotationH float32 = 60
)

// AnnotationWidget renders a documentation text box, optionally connected to
// its anchor element (node, edge, or group) by a thin dashed leader line.
// Text is rendered one canvas.Text per newline-separated line — there is no
// soft wrap; users break lines explicitly.
type AnnotationWidget struct {
	widget.BaseWidget

	AnnotationID string
	Selected     bool

	handles []*HandleWidget // corner resize handles, visible while selected

	// Leader-line target in widget-local coordinates, valid when leaderShown.
	// Updated by the canvas renderer after nodes/groups are positioned.
	leaderTarget geo.Position
	leaderShown  bool

	dagCanvas    *DAGCanvas
	dragging     bool
	dragStartPos [2]float32
}

var (
	_ core.Widget             = (*AnnotationWidget)(nil)
	_ event.Draggable         = (*AnnotationWidget)(nil)
	_ event.Tappable          = (*AnnotationWidget)(nil)
	_ event.SecondaryTappable = (*AnnotationWidget)(nil)
	_ core.DragImageProvider  = (*AnnotationWidget)(nil)
)

// DragImage returns nil (the box moves live); implementing DragImageProvider
// gives the grabbing cursor while the annotation is being dragged.
func (a *AnnotationWidget) DragImage() core.CanvasObject { return nil }

// NewAnnotationWidget creates a widget for a dag.Annotation.
func NewAnnotationWidget(a *dag.Annotation, dc *DAGCanvas) *AnnotationWidget {
	aw := &AnnotationWidget{
		AnnotationID: a.ID,
		dagCanvas:    dc,
	}
	aw.handles = newHandleSet(aw, annotationHandleDirs)
	aw.ExtendBaseWidget(aw)
	return aw
}

// annotation returns the underlying dag.Annotation, or nil if removed.
func (a *AnnotationWidget) annotation() *dag.Annotation {
	if a.dagCanvas == nil {
		return nil
	}
	return a.dagCanvas.Graph.Annotations[a.AnnotationID]
}

// contentMinSize returns the smallest box that fits the annotation text, in
// graph units. The text is measured at the size the renderer draws it
// (annotationTextSize × zoom) and converted back to graph units: text does not
// scale exactly with its size (a fractional size renders like the next whole
// pixel size), so measuring once at the base size and scaling the box by the
// zoom let the text overflow at some zoom levels.
func (a *AnnotationWidget) contentMinSize() geo.Size {
	w, h := annotationMinW, annotationMinH
	ann := a.annotation()
	if ann == nil {
		return geo.NewSize(w, h)
	}
	z := a.zoom()
	lines := strings.Split(ann.Text, "\n")
	lineH := annotationTextSize + 4 // keep in sync with annotationRenderer.Layout
	needH := float32(len(lines))*lineH + 2*annotationPadding
	needW := float32(0)
	for _, line := range lines {
		lw := bellui.MeasureText(line, annotationTextSize*z, bellui.TextStyle{}).Width / z
		if lw > needW {
			needW = lw
		}
	}
	needW += 2 * annotationPadding
	return geo.NewSize(max32(w, needW), max32(h, needH))
}

// effectiveSize returns the box size in graph units: the stored size grown as
// needed so the text never overflows the box.
func (a *AnnotationWidget) effectiveSize() geo.Size {
	min := a.contentMinSize()
	ann := a.annotation()
	if ann == nil {
		return min
	}
	return geo.NewSize(
		max32(ann.Size[0], min.Width),
		max32(ann.Size[1], min.Height),
	)
}

func (a *AnnotationWidget) MinSize() geo.Size {
	return geo.NewSize(0, 0) // sized by the canvas renderer
}

func (a *AnnotationWidget) zoom() float32 {
	if a.dagCanvas != nil {
		return a.dagCanvas.Zoom
	}
	return 1.0
}

// updateLeader recomputes the leader-line endpoint from the anchor element's
// current screen geometry. Called by the canvas renderer after nodes and
// groups have been positioned.
func (a *AnnotationWidget) updateLeader() {
	a.leaderShown = false
	ann := a.annotation()
	c := a.dagCanvas
	if ann == nil || c == nil || ann.AnchorKind == dag.AnchorNone {
		a.Refresh()
		return
	}
	boxRect := geo.Rectangle{Pos: a.Position(), Size: a.Size()}
	target, ok := c.anchorScreenPoint(ann.AnchorKind, ann.AnchorID, rectCenter(boxRect))
	if !ok {
		a.Refresh()
		return
	}
	// Convert to widget-local coordinates; children may render outside the
	// widget bounds (no clipping), same principle the edge widgets rely on.
	a.leaderTarget = geo.NewPos(target.X-boxRect.Pos.X, target.Y-boxRect.Pos.Y)
	a.leaderShown = true
	a.Refresh()
}

// Dragged moves the annotation in graph coordinates.
func (a *AnnotationWidget) Dragged(ev *event.DragEvent) {
	ann := a.annotation()
	if ann == nil || a.dagCanvas == nil || !a.dagCanvas.Capabilities.MoveResize {
		return
	}
	if !a.dragging {
		a.dragging = true
		a.dragStartPos = ann.Position
	}
	zoom := a.dagCanvas.Zoom
	ann.Position[0] += ev.Dragged.DX / zoom
	ann.Position[1] += ev.Dragged.DY / zoom
	a.dagCanvas.Refresh()
}

func (a *AnnotationWidget) DragEnd() {
	if !a.dragging {
		return
	}
	a.dragging = false
	ann := a.annotation()
	if ann == nil || ann.Position == a.dragStartPos {
		return
	}
	// Recorded without executing — the drag already moved the box.
	a.dagCanvas.pushDone(&MoveAnnotationCommand{
		Canvas:       a.dagCanvas,
		AnnotationID: a.AnnotationID,
		OldPos:       a.dragStartPos,
		NewPos:       ann.Position,
	})
}

// Tapped selects this annotation.
func (a *AnnotationWidget) Tapped(*event.PointEvent) {
	if a.dagCanvas != nil {
		a.dagCanvas.SelectAnnotation(a.AnnotationID)
	}
}

// TappedSecondary selects this annotation like a primary tap — a context menu
// acts on the element under the pointer — then fires
// OnAnnotationSecondaryTapped.
func (a *AnnotationWidget) TappedSecondary(ev *event.PointEvent) {
	if a.dagCanvas == nil {
		return
	}
	a.dagCanvas.SelectAnnotation(a.AnnotationID)
	if f := a.dagCanvas.OnAnnotationSecondaryTapped; f != nil {
		f(a.AnnotationID, ev)
	}
}

func (a *AnnotationWidget) CreateRenderer() core.WidgetRenderer {
	return newAnnotationRenderer(a)
}

// --- resizable interface ---

func (a *AnnotationWidget) resizeCanvas() *DAGCanvas { return a.dagCanvas }

func (a *AnnotationWidget) storedGeometry() (pos, size [2]float32, ok bool) {
	ann := a.annotation()
	if ann == nil {
		return pos, size, false
	}
	return ann.Position, ann.Size, true
}

func (a *AnnotationWidget) setStoredGeometry(pos, size [2]float32) {
	if ann := a.annotation(); ann != nil {
		ann.Position = pos
		ann.Size = size
	}
}

func (a *AnnotationWidget) effectiveGraphSize() geo.Size { return a.effectiveSize() }

// handlesVisible reports whether the resize handles should show: selected,
// and the canvas allows move/resize. Shared by the renderer's Refresh and
// the canvas-side affordance sync (see dagCanvasRenderer.Layout).
func (a *AnnotationWidget) handlesVisible() bool {
	return a.Selected && (a.dagCanvas == nil || a.dagCanvas.Capabilities.MoveResize)
}

// minGraphSize includes the text extent so resize handles cannot shrink the
// box below its content.
func (a *AnnotationWidget) minGraphSize() geo.Size {
	return a.contentMinSize()
}

func (a *AnnotationWidget) pushResizeCommand(oldPos, oldSize, newPos, newSize [2]float32) {
	a.dagCanvas.pushDone(&ResizeAnnotationCommand{
		Canvas:       a.dagCanvas,
		AnnotationID: a.AnnotationID,
		OldPos:       oldPos,
		NewPos:       newPos,
		OldSize:      oldSize,
		NewSize:      newSize,
	})
}

var _ resizable = (*AnnotationWidget)(nil)

// --- renderer ---

type annotationRenderer struct {
	ann    *AnnotationWidget
	bg     *canvas.Rectangle
	leader *canvas.Path
	lines  []*canvas.Text // one per newline-separated text line
}

func newAnnotationRenderer(a *AnnotationWidget) *annotationRenderer {
	r := &annotationRenderer{
		ann:    a,
		bg:     &canvas.Rectangle{},
		leader: &canvas.Path{},
	}
	r.leader.FillColor = color.Transparent
	return r
}

func (r *annotationRenderer) Layout(size geo.Size) {
	a := r.ann
	z := a.zoom()

	r.bg.Move(geo.NewPos(0, 0))
	r.bg.Resize(size)
	r.bg.CornerRadius = 6 * z

	pad := annotationPadding * z
	lineH := (annotationTextSize + 4) * z
	for i, txt := range r.lines {
		txt.TextSize = annotationTextSize * z
		txt.Move(geo.NewPos(pad, pad+float32(i)*lineH))
		txt.Resize(geo.NewSize(size.Width-2*pad, lineH))
	}

	hs := handleSize * z
	for _, hw := range a.handles {
		fx := float32(hw.dirX+1) / 2
		fy := float32(hw.dirY+1) / 2
		hw.Move(geo.NewPos(fx*size.Width-hs/2, fy*size.Height-hs/2))
		hw.Resize(geo.NewSquareSize(hs))
	}
}

func (r *annotationRenderer) MinSize() geo.Size {
	return geo.NewSize(0, 0)
}

func (r *annotationRenderer) Refresh() {
	a := r.ann
	th := a.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	z := a.zoom()

	if a.Selected {
		r.bg.StrokeColor = th.Color(theme.ColorNamePrimary, v)
		r.bg.StrokeWidth = 2 * z
	} else {
		r.bg.StrokeColor = th.Color(theme.ColorNameSeparator, v)
		r.bg.StrokeWidth = 1 * z
	}
	r.bg.FillColor = withAlpha(th.Color(theme.ColorNameButton, v), 230)

	// Rebuild text lines when the text changed.
	text := ""
	if ann := a.annotation(); ann != nil {
		text = ann.Text
	}
	wanted := strings.Split(text, "\n")
	if len(wanted) != len(r.lines) {
		r.lines = r.lines[:0]
		for range wanted {
			r.lines = append(r.lines, &canvas.Text{TextSize: annotationTextSize})
		}
		// New objects — recompute their placement immediately.
		r.Layout(a.Size())
	}
	fg := th.Color(theme.ColorNameForeground, v)
	for i, txt := range r.lines {
		txt.Text = wanted[i]
		txt.Color = fg
		txt.Refresh()
	}

	// Leader line from the box border toward the anchor.
	r.leader.Reset()
	if a.leaderShown {
		boxRect := geo.Rectangle{Pos: geo.NewPos(0, 0), Size: a.Size()}
		start := rectBorderPoint(boxRect, a.leaderTarget)
		r.leader.MoveTo(start.X, start.Y)
		r.leader.LineTo(a.leaderTarget.X, a.leaderTarget.Y)
		r.leader.StrokeColor = withAlpha(th.Color(theme.ColorNameForeground, v), 130)
		r.leader.StrokeWidth = 1 * z
		r.leader.DashPattern = []float32{4 * z, 3 * z}
		r.leader.Show()
	} else {
		r.leader.Hide()
	}
	r.leader.Refresh()
	r.bg.Refresh()

	setHandlesVisible(a.handles, a.handlesVisible())
	for _, hw := range a.handles {
		hw.Refresh()
	}
}

func (r *annotationRenderer) Objects() []core.CanvasObject {
	objs := make([]core.CanvasObject, 0, 2+len(r.lines)+len(r.ann.handles))
	objs = append(objs, r.leader, r.bg)
	for _, txt := range r.lines {
		objs = append(objs, txt)
	}
	for _, hw := range r.ann.handles {
		objs = append(objs, hw)
	}
	return objs
}

func (r *annotationRenderer) Destroy() {}
