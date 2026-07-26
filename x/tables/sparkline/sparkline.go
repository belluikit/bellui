// Package sparkline provides in-cell sparkline chart widgets for the data grid.
package sparkline

import (
	"fmt"
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// Compile-time interface checks.
var (
	_ event.Hoverable = (*Sparkline)(nil)
)

// Type determines the sparkline chart type.
type Type int

const (
	// Line draws a line chart.
	Line Type = iota
	// Area draws a line chart with a filled area below.
	Area
	// Bar draws horizontal bars.
	Bar
	// Column draws vertical columns.
	Column
)

// MarkerShape determines the shape drawn at each data point.
type MarkerShape int

const (
	MarkerCircle MarkerShape = iota
	MarkerSquare
	MarkerDiamond
)

// Options configures the appearance of a sparkline.
type Options struct {
	// Line / stroke
	Stroke      color.Color
	StrokeWidth float32

	// Fill (area under line, or bar/column fill)
	Fill color.Color

	// Negative value color (for bar/column charts)
	NegativeColor color.Color

	// Markers at data points
	ShowMarkers bool
	MarkerSize  float32
	MarkerShape MarkerShape
	MarkerFill  color.Color

	// Axis
	ShowAxis  bool
	AxisColor color.Color

	// Domain scaling: fix the min/max range instead of auto-scaling
	DomainMin *float64
	DomainMax *float64

	// Padding around the chart area
	Padding float32

	// Highlight a specific data point (e.g. on hover)
	HighlightIndex int // -1 = none
	HighlightColor color.Color
	HighlightSize  float32

	// ItemStyler overrides per-point colors
	ItemStyler func(index int, value float64) color.Color
}

// Sparkline is a Fyne widget that renders a mini chart inside a cell.
type Sparkline struct {
	widget.BaseWidget

	ChartType Type
	Data      []float64
	Opts      Options

	// cache
	lastW, lastH int
	dirty        bool

	// tooltip state
	tooltipPopup *widget.PopUp
	hovered      bool
}

// New creates a new Sparkline widget.
func New(chartType Type, data []float64, opts ...func(*Options)) *Sparkline {
	s := &Sparkline{
		ChartType: chartType,
		Data:      data,
		dirty:     true,
		Opts: Options{
			Stroke:         color.NRGBA{R: 33, G: 150, B: 243, A: 255},
			StrokeWidth:    1.5,
			Fill:           color.NRGBA{R: 33, G: 150, B: 243, A: 40},
			NegativeColor:  color.NRGBA{R: 244, G: 67, B: 54, A: 255},
			Padding:        2,
			HighlightIndex: -1,
			HighlightColor: color.NRGBA{R: 255, G: 152, B: 0, A: 255},
			HighlightSize:  4,
			AxisColor:      color.NRGBA{R: 128, G: 128, B: 128, A: 100},
			MarkerFill:     color.NRGBA{R: 33, G: 150, B: 243, A: 255},
		},
	}
	for _, opt := range opts {
		opt(&s.Opts)
	}
	s.ExtendBaseWidget(s)
	return s
}

// NewLine creates a line sparkline.
func NewLine(data []float64, opts ...func(*Options)) *Sparkline {
	return New(Line, data, opts...)
}

// NewArea creates an area sparkline.
func NewArea(data []float64, opts ...func(*Options)) *Sparkline {
	return New(Area, data, opts...)
}

// NewBar creates a bar sparkline.
func NewBar(data []float64, opts ...func(*Options)) *Sparkline {
	return New(Bar, data, opts...)
}

// NewColumn creates a column sparkline.
func NewColumn(data []float64, opts ...func(*Options)) *Sparkline {
	return New(Column, data, opts...)
}

// SetData replaces the sparkline data and refreshes.
func (s *Sparkline) SetData(data []float64) {
	s.Data = data
	s.dirty = true
	s.Refresh()
}

// SetHighlight highlights a specific data point by index. Pass -1 to clear.
func (s *Sparkline) SetHighlight(index int) {
	s.Opts.HighlightIndex = index
	s.dirty = true
	s.Refresh()
}

// CreateRenderer returns the sparkline renderer.
func (s *Sparkline) CreateRenderer() core.WidgetRenderer {
	s.ExtendBaseWidget(s)
	return newSparklineRenderer(s)
}

// MinSize returns the minimum size for the sparkline.
func (s *Sparkline) MinSize() geo.Size {
	return geo.NewSize(50, 20)
}

// --- Hover / Tooltip ---

// HoverIn is called when the mouse enters the sparkline.
func (s *Sparkline) HoverIn(e *event.HoverEvent) {
	s.hovered = true
	s.updateHoverPoint(e.Position)
}

// HoverMoved is called when the mouse moves over the sparkline.
func (s *Sparkline) HoverMoved(e *event.HoverEvent) {
	s.updateHoverPoint(e.Position)
}

// HoverOut is called when the mouse leaves the sparkline.
func (s *Sparkline) HoverOut() {
	s.hovered = false
	s.Opts.HighlightIndex = -1
	s.dirty = true
	s.dismissTooltip()
	s.Refresh()
}

func (s *Sparkline) updateHoverPoint(pos geo.Position) {
	if len(s.Data) < 2 {
		return
	}

	// Map mouse X to nearest data point index
	size := s.Size()
	pad := s.Opts.Padding
	drawW := size.Width - pad*2
	if drawW <= 0 {
		return
	}

	relX := pos.X - pad
	if relX < 0 {
		relX = 0
	}
	if relX > drawW {
		relX = drawW
	}

	idx := int(relX/drawW*float32(len(s.Data)-1) + 0.5)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(s.Data) {
		idx = len(s.Data) - 1
	}

	if s.Opts.HighlightIndex != idx {
		s.Opts.HighlightIndex = idx
		s.dirty = true
		s.Refresh()
	}

	// Show tooltip
	s.showTooltip(idx, pos)
}

func (s *Sparkline) showTooltip(idx int, pos geo.Position) {
	if idx < 0 || idx >= len(s.Data) {
		return
	}

	c := bellui.CurrentApp().Driver().CanvasForObject(s)
	if c == nil {
		return
	}

	text := fmt.Sprintf("[%d] %.2f", idx, s.Data[idx])

	if s.tooltipPopup != nil {
		// Update existing popup content
		s.tooltipPopup.Hide()
	}

	label := canvas.NewText(text, nil)
	label.TextSize = 11
	bg := canvas.NewRectangle(color.NRGBA{R: 50, G: 50, B: 50, A: 220})
	label.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 255}

	content := &bellui.Container{}
	content.Objects = []core.CanvasObject{bg, label}
	content.Resize(geo.NewSize(label.MinSize().Width+12, label.MinSize().Height+6))
	bg.Resize(content.Size())
	label.Move(geo.NewPos(6, 3))

	popup := widget.NewPopUp(content, c)

	// Position above the hover point
	absPos := bellui.CurrentApp().Driver().AbsolutePositionForObject(s)
	tooltipPos := geo.NewPos(absPos.X+pos.X+10, absPos.Y+pos.Y-20)
	popup.ShowAtPosition(tooltipPos)
	s.tooltipPopup = popup
}

func (s *Sparkline) dismissTooltip() {
	if s.tooltipPopup != nil {
		s.tooltipPopup.Hide()
		s.tooltipPopup = nil
	}
}

// --- Functional option helpers ---

// WithStroke sets the stroke color.
func WithStroke(c color.Color) func(*Options) {
	return func(o *Options) { o.Stroke = c }
}

// WithFill sets the fill color.
func WithFill(c color.Color) func(*Options) {
	return func(o *Options) { o.Fill = c }
}

// WithMarkers enables data point markers.
func WithMarkers(size float32) func(*Options) {
	return func(o *Options) {
		o.ShowMarkers = true
		o.MarkerSize = size
	}
}

// WithAxis enables the zero-line axis.
func WithAxis() func(*Options) {
	return func(o *Options) { o.ShowAxis = true }
}

// WithDomain sets a fixed min/max domain range.
func WithDomain(min, max float64) func(*Options) {
	return func(o *Options) {
		o.DomainMin = &min
		o.DomainMax = &max
	}
}
