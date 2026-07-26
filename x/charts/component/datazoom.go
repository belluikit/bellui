package component

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/charts/interact"
)

// DataZoomSlider is a range-selection slider for controlling the visible data window.
type DataZoomSlider struct {
	widget.BaseWidget

	window   interact.DataWindow
	onChange func(interact.DataWindow)

	// Drag state
	dragging   dragTarget
	dragStartX float32
	dragStartW interact.DataWindow
}

type dragTarget int

const (
	dragNone dragTarget = iota
	dragLeftHandle
	dragRightHandle
	dragBody
)

var (
	_ core.Widget     = (*DataZoomSlider)(nil)
	_ event.Tappable  = (*DataZoomSlider)(nil)
	_ event.Draggable = (*DataZoomSlider)(nil)
	_ event.Hoverable = (*DataZoomSlider)(nil)
)

// NewDataZoomSlider creates a DataZoomSlider with a callback fired when the window changes.
func NewDataZoomSlider(onChange func(interact.DataWindow)) *DataZoomSlider {
	s := &DataZoomSlider{
		window:   interact.FullWindow(),
		onChange: onChange,
	}
	s.ExtendBaseWidget(s)
	return s
}

// SetWindow updates the slider's displayed window.
func (s *DataZoomSlider) SetWindow(w interact.DataWindow) {
	s.window = w
	s.Refresh()
}

// Window returns the current data window.
func (s *DataZoomSlider) Window() interact.DataWindow {
	return s.window
}

// CreateRenderer returns the widget renderer for the slider.
func (s *DataZoomSlider) CreateRenderer() core.WidgetRenderer {
	return &dataZoomRenderer{slider: s}
}

// MinSize returns the minimum size for the slider widget.
func (s *DataZoomSlider) MinSize() geo.Size {
	return geo.NewSize(100, 30)
}

// --- Interaction ---

const handleWidth = float32(6)

// Tapped is a no-op; the slider only responds to drag events.
func (s *DataZoomSlider) Tapped(_ *event.PointEvent) {}

// Dragged handles drag events on the left handle, right handle, or body of the selection.
func (s *DataZoomSlider) Dragged(e *event.DragEvent) {
	size := s.Size()
	if s.dragging == dragNone {
		// Determine what was grabbed
		leftX := float32(s.window.Start) * size.Width
		rightX := float32(s.window.End) * size.Width

		if e.Position.X >= leftX-handleWidth && e.Position.X <= leftX+handleWidth {
			s.dragging = dragLeftHandle
		} else if e.Position.X >= rightX-handleWidth && e.Position.X <= rightX+handleWidth {
			s.dragging = dragRightHandle
		} else if e.Position.X > leftX && e.Position.X < rightX {
			s.dragging = dragBody
		} else {
			return
		}
		s.dragStartX = e.Position.X
		s.dragStartW = s.window
	}

	dx := float64(e.Position.X-s.dragStartX) / float64(size.Width)

	switch s.dragging {
	case dragLeftHandle:
		s.window.Start = s.dragStartW.Start + dx
		if s.window.Start < 0 {
			s.window.Start = 0
		}
		if s.window.Start > s.window.End-0.01 {
			s.window.Start = s.window.End - 0.01
		}
	case dragRightHandle:
		s.window.End = s.dragStartW.End + dx
		if s.window.End > 1 {
			s.window.End = 1
		}
		if s.window.End < s.window.Start+0.01 {
			s.window.End = s.window.Start + 0.01
		}
	case dragBody:
		span := s.dragStartW.Span()
		s.window.Start = s.dragStartW.Start + dx
		s.window.End = s.window.Start + span
		if s.window.Start < 0 {
			s.window.Start = 0
			s.window.End = span
		}
		if s.window.End > 1 {
			s.window.End = 1
			s.window.Start = 1 - span
		}
	}

	if s.onChange != nil {
		s.onChange(s.window)
	}
	s.Refresh()
}

// DragEnd resets the drag target when the user releases the pointer.
func (s *DataZoomSlider) DragEnd() {
	s.dragging = dragNone
}

// MouseIn is a no-op (satisfies event.Hoverable).
func (s *DataZoomSlider) HoverIn(_ *event.HoverEvent) {}

// MouseMoved is a no-op (satisfies event.Hoverable).
func (s *DataZoomSlider) HoverMoved(_ *event.HoverEvent) {}

// MouseOut is a no-op (satisfies event.Hoverable).
func (s *DataZoomSlider) HoverOut() {}

// --- Renderer ---

type dataZoomRenderer struct {
	slider  *DataZoomSlider
	objects []core.CanvasObject
}

func (r *dataZoomRenderer) Layout(size geo.Size) {
	r.rebuild(size)
}

func (r *dataZoomRenderer) MinSize() geo.Size {
	return geo.NewSize(100, 30)
}

func (r *dataZoomRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *dataZoomRenderer) Refresh() {
	r.rebuild(r.slider.Size())
}

func (r *dataZoomRenderer) Destroy() {}

func (r *dataZoomRenderer) rebuild(size geo.Size) {
	if size.Width <= 0 || size.Height <= 0 {
		return
	}

	var objects []core.CanvasObject
	w := r.slider.window

	// Track background
	track := canvas.NewRectangle(color.NRGBA{R: 50, G: 50, B: 50, A: 255})
	track.CornerRadius = 3
	track.Move(geo.NewPos(0, 0))
	track.Resize(size)
	objects = append(objects, track)

	// Left mask (unselected region)
	leftW := float32(w.Start) * size.Width
	if leftW > 0 {
		leftMask := canvas.NewRectangle(color.NRGBA{R: 30, G: 30, B: 30, A: 180})
		leftMask.Move(geo.NewPos(0, 0))
		leftMask.Resize(geo.NewSize(leftW, size.Height))
		objects = append(objects, leftMask)
	}

	// Right mask
	rightX := float32(w.End) * size.Width
	rightW := size.Width - rightX
	if rightW > 0 {
		rightMask := canvas.NewRectangle(color.NRGBA{R: 30, G: 30, B: 30, A: 180})
		rightMask.Move(geo.NewPos(rightX, 0))
		rightMask.Resize(geo.NewSize(rightW, size.Height))
		objects = append(objects, rightMask)
	}

	// Selected range highlight
	selX := float32(w.Start) * size.Width
	selW := float32(w.End-w.Start) * size.Width
	sel := canvas.NewRectangle(color.NRGBA{R: 91, G: 143, B: 249, A: 40})
	sel.StrokeColor = color.NRGBA{R: 91, G: 143, B: 249, A: 180}
	sel.StrokeWidth = 1
	sel.Move(geo.NewPos(selX, 0))
	sel.Resize(geo.NewSize(selW, size.Height))
	objects = append(objects, sel)

	// Left handle
	lh := canvas.NewRectangle(color.NRGBA{R: 91, G: 143, B: 249, A: 255})
	lh.CornerRadius = 2
	lh.Move(geo.NewPos(selX-handleWidth/2, 2))
	lh.Resize(geo.NewSize(handleWidth, size.Height-4))
	objects = append(objects, lh)

	// Right handle
	rh := canvas.NewRectangle(color.NRGBA{R: 91, G: 143, B: 249, A: 255})
	rh.CornerRadius = 2
	rh.Move(geo.NewPos(selX+selW-handleWidth/2, 2))
	rh.Resize(geo.NewSize(handleWidth, size.Height-4))
	objects = append(objects, rh)

	r.objects = objects
}
