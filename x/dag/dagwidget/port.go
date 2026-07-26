package dagwidget

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/dag/dag"
)

const portSize float32 = 12

// PortWidget represents a connection point on a node.
type PortWidget struct {
	widget.BaseWidget

	PortID    dag.PortID
	Direction dag.PortDirection
	TypeTag   string
	Connected bool
	Hovered   bool
	ValidDrop bool

	dagCanvas *DAGCanvas
}

var (
	_ core.Widget            = (*PortWidget)(nil)
	_ event.Draggable        = (*PortWidget)(nil)
	_ desktop.Cursorable     = (*PortWidget)(nil)
	_ core.DragImageProvider = (*PortWidget)(nil)
)

// Cursor shows a crosshair over ports to hint that a connection can be drawn
// from here.
func (p *PortWidget) Cursor() desktop.Cursor {
	return desktop.CrosshairCursor
}

// DragImage returns nil — the canvas already draws the ghost bezier — but
// implementing DragImageProvider makes the driver show the grabbing cursor
// for the duration of a connection drag.
func (p *PortWidget) DragImage() core.CanvasObject { return nil }

// NewPortWidget creates a new connection port.
func NewPortWidget(pid dag.PortID, dir dag.PortDirection, typeTag string, dc *DAGCanvas) *PortWidget {
	p := &PortWidget{
		PortID:    pid,
		Direction: dir,
		TypeTag:   typeTag,
		dagCanvas: dc,
	}
	p.ExtendBaseWidget(p)
	return p
}

func (p *PortWidget) CreateRenderer() core.WidgetRenderer {
	circle := &canvas.Circle{
		StrokeColor: color.Transparent,
		StrokeWidth: 2,
		FillColor:   color.Transparent,
	}
	ring := &canvas.Circle{
		StrokeColor: color.Transparent,
		StrokeWidth: 2,
		FillColor:   color.Transparent,
	}
	ring.Hide()
	r := &portRenderer{
		port:   p,
		circle: circle,
		ring:   ring,
	}
	r.applyTheme()
	return r
}

func (p *PortWidget) MinSize() geo.Size {
	return geo.NewSquareSize(portSize)
}

// Dragged initiates or continues a connection from this port.
// - Output port: starts a new connection drag.
// - Connected input port: detaches the incoming edge and reroutes from the original source.
func (p *PortWidget) Dragged(ev *event.DragEvent) {
	if p.dagCanvas == nil || !p.dagCanvas.Capabilities.ConnectEdges {
		return
	}

	if p.dagCanvas.dragState != dragConnecting {
		if p.Direction == dag.PortOutput {
			// Start a new connection from this output
			p.dagCanvas.startConnection(p.PortID, p.screenCenter())
		} else if p.Direction == dag.PortInput && p.Connected {
			// Detach the incoming edge and reroute from the original source
			p.dagCanvas.detachAndReroute(p.PortID)
		}
	}

	if p.dagCanvas.dragState == dragConnecting {
		p.dagCanvas.updatePendingConnection(ev.AbsolutePosition)
	}
}

func (p *PortWidget) DragEnd() {
	if p.dagCanvas != nil && p.dagCanvas.dragState == dragConnecting {
		p.dagCanvas.finishConnection()
	}
}

// screenCenter returns this port's center in screen coordinates relative to the DAGCanvas.
func (p *PortWidget) screenCenter() geo.Position {
	driver := bellui.CurrentApp().Driver()
	absPort := driver.AbsolutePositionForObject(p)
	absCanvas := driver.AbsolutePositionForObject(p.dagCanvas)
	return geo.NewPos(
		absPort.X-absCanvas.X+p.Size().Width/2,
		absPort.Y-absCanvas.Y+p.Size().Height/2,
	)
}

func (p *PortWidget) zoom() float32 {
	if p.dagCanvas != nil {
		return p.dagCanvas.Zoom
	}
	return 1.0
}

type portRenderer struct {
	port   *PortWidget
	circle *canvas.Circle
	ring   *canvas.Circle
}

func (r *portRenderer) Layout(size geo.Size) {
	r.circle.Resize(size)
	r.circle.Move(geo.NewPos(0, 0))
	// Ring is slightly larger for highlight
	pad := float32(2) * r.port.zoom()
	r.ring.Move(geo.NewPos(-pad, -pad))
	r.ring.Resize(geo.NewSize(size.Width+pad*2, size.Height+pad*2))
}

func (r *portRenderer) MinSize() geo.Size {
	return geo.NewSquareSize(portSize)
}

func (r *portRenderer) Refresh() {
	r.applyTheme()
	canvas.Refresh(r.port)
}

func (r *portRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.ring, r.circle}
}

func (r *portRenderer) Destroy() {}

func (r *portRenderer) applyTheme() {
	th := r.port.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	var strokeColor color.Color = th.Color(theme.ColorNameOutline, v)
	var fillColor color.Color = color.Transparent

	if r.port.Connected {
		fillColor = th.Color(theme.ColorNamePrimary, v)
		strokeColor = fillColor
	}
	if r.port.Hovered {
		fillColor = withAlpha(th.Color(theme.ColorNamePrimary, v), 80)
	}

	z := r.port.zoom()
	r.circle.FillColor = fillColor
	r.circle.StrokeColor = strokeColor
	r.circle.StrokeWidth = 2 * z
	r.circle.Refresh()

	if r.port.ValidDrop {
		r.ring.StrokeColor = th.Color(theme.ColorNamePrimary, v)
		r.ring.StrokeWidth = 2 * z
		r.ring.Show()
		r.ring.Refresh()
	} else {
		r.ring.Hide()
	}
}

func withAlpha(c color.Color, alpha uint8) color.Color {
	r, g, b, _ := c.RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: alpha}
}
