package widget

import (
	"fmt"
	"image/color"
	"math"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internaltheme "github.com/belluikit/bellui/internal/theme"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
)

var (
	_ event.Draggable = (*RangeSlider)(nil)
	_ event.Tappable  = (*RangeSlider)(nil)
	_ event.Hoverable = (*RangeSlider)(nil)
	_ core.Accessible = (*RangeSlider)(nil)
)

// AccessibleRole returns [core.RoleGroup]: a range slider is a container of two
// independently adjustable thumbs — the low and high sliders — each exposed as
// its own accessible child ([rangeSliderThumb], #810) rather than a single
// slider. Mobile screen readers flatten the group away and surface the two thumb
// sliders directly; desktop screen readers present them within the group.
func (s *RangeSlider) AccessibleRole() core.AccessibleRole { return core.RoleGroup }

// AccessibleLabel returns an empty string; the group has no intrinsic label and
// each thumb carries its own.
func (s *RangeSlider) AccessibleLabel() string { return "" }

// a11yStep is the amount one increment/decrement action moves a thumb. It
// honours an explicit Step, falling back to a hundredth of the range (or 1 for a
// degenerate range) so a thumb is still adjustable when Step is unset.
func (s *RangeSlider) a11yStep() float64 {
	if s.Step > 0 {
		return s.Step
	}
	if s.Max > s.Min {
		return (s.Max - s.Min) / 100
	}
	return 1
}

// adjustThumb moves the low or high thumb by delta, clamping to the range and
// keeping the low thumb at or below the high thumb. A screen-reader adjustment
// is a complete interaction, so it reports both OnChanged and OnChangeEnded.
func (s *RangeSlider) adjustThumb(high bool, delta float64) {
	lastLow, lastHigh := s.LowValue, s.HighValue
	if high {
		s.HighValue = s.clamp(s.HighValue + delta)
		if s.HighValue < s.LowValue {
			s.HighValue = s.LowValue
		}
	} else {
		s.LowValue = s.clamp(s.LowValue + delta)
		if s.LowValue > s.HighValue {
			s.LowValue = s.HighValue
		}
	}
	s.valuesChanged(lastLow, lastHigh)
	s.fireChangeEnded()
}

// rangeSliderThumb is an accessibility-only proxy for one thumb of a
// [RangeSlider], exposing the low and high thumbs as two independently
// adjustable sliders (#810). The earlier single-node model moved an "active"
// thumb and switched it with the primary activation, but Android/TalkBack offers
// no double-tap-activate on a seek control, so the switch was unreachable and the
// second thumb could not be adjusted. Two child sliders need no mode switch and
// are cleaner on every platform. The proxy has no visual of its own — the parent
// renderer paints the thumb circle — and implements no pointer interface, so
// drags and taps fall through to the RangeSlider; the parent renderer positions
// it over its thumb.
type rangeSliderThumb struct {
	BaseWidget
	slider *RangeSlider
	high   bool
}

var (
	_ core.Accessible         = (*rangeSliderThumb)(nil)
	_ core.AccessibleValuer   = (*rangeSliderThumb)(nil)
	_ core.AccessibleActioner = (*rangeSliderThumb)(nil)
)

// AccessibleRole returns [core.RoleSlider]: each thumb is its own slider.
func (t *rangeSliderThumb) AccessibleRole() core.AccessibleRole { return core.RoleSlider }

// AccessibleLabel names the thumb so a screen reader distinguishes the two.
func (t *rangeSliderThumb) AccessibleLabel() string {
	if t.high {
		return lang.L("Maximum")
	}
	return lang.L("Minimum")
}

// AccessibleValue returns this thumb's current value.
func (t *rangeSliderThumb) AccessibleValue() string {
	return fmt.Sprintf("%.1f", t.value())
}

// AccessibleRange returns the slider's min/max and this thumb's value.
func (t *rangeSliderThumb) AccessibleRange() (min, max, current float64) {
	return t.slider.Min, t.slider.Max, t.value()
}

// AccessibleActions lets a screen reader adjust this thumb directly, with no
// active-thumb mode switch.
func (t *rangeSliderThumb) AccessibleActions() []core.AccessibleAction {
	return []core.AccessibleAction{
		{Name: core.ActionIncrement, Description: "Increase this value"},
		{Name: core.ActionDecrement, Description: "Decrease this value"},
	}
}

// PerformAccessibleAction adjusts this thumb by one step.
func (t *rangeSliderThumb) PerformAccessibleAction(name string) {
	switch name {
	case core.ActionIncrement:
		t.slider.adjustThumb(t.high, t.slider.a11yStep())
	case core.ActionDecrement:
		t.slider.adjustThumb(t.high, -t.slider.a11yStep())
	}
}

func (t *rangeSliderThumb) value() float64 {
	if t.high {
		return t.slider.HighValue
	}
	return t.slider.LowValue
}

// CreateRenderer returns an empty transparent renderer — the proxy is an
// accessibility node only; the RangeSlider's renderer paints the thumb.
func (t *rangeSliderThumb) CreateRenderer() core.WidgetRenderer {
	t.ExtendBaseWidget(t)
	return NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

// RangeSlider is a widget with two thumbs for selecting a value range.
type RangeSlider struct {
	BaseWidget

	LowValue  float64
	HighValue float64
	Min       float64
	Max       float64
	Step      float64

	Orientation Orientation
	OnChanged   func(low, high float64) `json:"-"`

	// OnChangeEnded is called once a change to the range has settled: the end of
	// a drag that moved a thumb, a tap, a screen-reader adjustment, or a call to
	// [RangeSlider.SetValues]. It is the commit signal a trim control (video
	// in/out points) acts on, as distinct from the stream of in-drag OnChanged
	// updates.
	//
	// It only fires when a thumb actually moved, so a press and release that
	// changed nothing reports neither callback — use [RangeSlider.Dragging] to
	// track drag state rather than inferring it from the callback pair.
	//
	// Since: 1.0
	OnChangeEnded func(low, high float64) `json:"-"`

	// OnHover is called with the value under the pointer as it moves across the
	// slider, without moving either thumb. It is the seam a media scrubber uses
	// to preview the frame at a position before committing a seek.
	//
	// The reported value is continuous — it is NOT snapped to Step, since a
	// preview wants the true pointer position and a consumer that wants the
	// snapped value can round it, while the reverse is not recoverable. It is
	// clamped to [Min, Max].
	//
	// Since: 1.0
	OnHover func(float64) `json:"-"`

	// OnHoverEnd is called when the pointer leaves the slider, so anything shown
	// in response to OnHover can be taken down again.
	//
	// Since: 1.0
	OnHoverEnd func() `json:"-"`

	hovered       bool
	draggingHigh  bool // true if dragging the high thumb
	dragging      bool
	pendingChange bool // true if a thumb moved since the last OnChangeEnded
}

// NewRangeSlider creates a range slider with the given bounds.
func NewRangeSlider(min, max float64) *RangeSlider {
	s := &RangeSlider{
		Min:       min,
		Max:       max,
		LowValue:  min,
		HighValue: max,
	}
	s.ExtendBaseWidget(s)
	return s
}

// SetValues updates both thumb positions, clamping each into [Min, Max],
// snapping to Step and swapping them if they arrive the wrong way round.
// It is safe to call from any goroutine.
//
// The change is reported as though the user made it: both OnChanged and
// OnChangeEnded fire, matching [Slider.SetValue]. For programmatic updates that
// should not look like user interaction — a player advancing a preview range
// many times a second — use [RangeSlider.SetValuesSilently].
func (s *RangeSlider) SetValues(low, high float64) {
	onMain(func() {
		lastLow, lastHigh := s.LowValue, s.HighValue
		s.assignValues(low, high)
		s.valuesChanged(lastLow, lastHigh)
		s.fireChangeEnded()
	})
}

// SetValuesSilently updates both thumb positions and redraws, without firing
// OnChanged or OnChangeEnded. It is safe to call from any goroutine.
//
// This is the path for updates the user did not cause, where every
// [RangeSlider.SetValues] call would otherwise be indistinguishable from the
// user dragging a thumb.
//
// Since: 1.0
func (s *RangeSlider) SetValuesSilently(low, high float64) {
	onMain(func() {
		lastLow, lastHigh := s.LowValue, s.HighValue
		s.assignValues(low, high)
		if lastLow == s.LowValue && lastHigh == s.HighValue {
			return
		}
		s.Refresh()
	})
}

// assignValues stores an ordered, clamped pair without reporting the change.
func (s *RangeSlider) assignValues(low, high float64) {
	if low > high {
		low, high = high, low
	}
	s.LowValue = s.clamp(low)
	s.HighValue = s.clamp(high)
}

// valuesChanged reports a move away from the given previous values. It fires
// nothing when neither thumb actually moved, so a drag that stays put — or a
// programmatic set to the values already held — is not reported as a change.
func (s *RangeSlider) valuesChanged(lastLow, lastHigh float64) {
	if lastLow == s.LowValue && lastHigh == s.HighValue {
		return
	}

	s.Refresh()

	s.pendingChange = true
	if s.OnChanged != nil {
		s.OnChanged(s.LowValue, s.HighValue)
	}
}

func (s *RangeSlider) fireChangeEnded() {
	if !s.pendingChange {
		return
	}
	s.pendingChange = false
	if s.OnChangeEnded != nil {
		s.OnChangeEnded(s.LowValue, s.HighValue)
	}
}

func (s *RangeSlider) Dragged(e *event.DragEvent) {
	ratio := s.getRatio(&e.PointEvent)
	val := s.Min + ratio*(s.Max-s.Min)

	if !s.dragging {
		// Determine which thumb to drag based on proximity.
		distLow := math.Abs(val - s.LowValue)
		distHigh := math.Abs(val - s.HighValue)
		s.draggingHigh = distHigh <= distLow
		s.dragging = true
	}

	lastLow, lastHigh := s.LowValue, s.HighValue
	if s.draggingHigh {
		s.HighValue = s.clamp(val)
		if s.HighValue < s.LowValue {
			s.HighValue = s.LowValue
		}
	} else {
		s.LowValue = s.clamp(val)
		if s.LowValue > s.HighValue {
			s.LowValue = s.HighValue
		}
	}

	s.valuesChanged(lastLow, lastHigh)
}

func (s *RangeSlider) DragEnd() {
	// Cleared unconditionally, and before the callback, for the reason
	// documented on [Slider.DragEnd]: a drag that never moved a thumb fires no
	// OnChangeEnded, so a consumer inferring drag state from the callback pair
	// would latch on forever. Dragging() must not have that hole.
	s.dragging = false
	s.fireChangeEnded()
}

// Dragging reports whether the user is currently dragging one of the thumbs,
// between the first Dragged event and DragEnd.
//
// This is the authoritative signal a media scrubber needs in order to suppress
// its own periodic position updates while the user is scrubbing — otherwise a
// thumb jumps back under the pointer on every tick. Inferring it from
// OnChanged/OnChangeEnded instead is unreliable: a press and release that does
// not move a thumb fires neither callback.
//
// Since: 1.0
func (s *RangeSlider) Dragging() bool {
	return s.dragging
}

// DraggingHigh reports which thumb an in-progress drag is moving: true for the
// high thumb, false for the low one. It is only meaningful while
// [RangeSlider.Dragging] reports true — afterwards it retains the thumb the
// last drag moved.
//
// Since: 1.0
func (s *RangeSlider) DraggingHigh() bool {
	return s.draggingHigh
}

func (s *RangeSlider) Tapped(e *event.PointEvent) {
	ratio := s.getRatio(e)
	val := s.Min + ratio*(s.Max-s.Min)

	lastLow, lastHigh := s.LowValue, s.HighValue
	distLow := math.Abs(val - s.LowValue)
	distHigh := math.Abs(val - s.HighValue)
	if distHigh <= distLow {
		s.HighValue = s.clamp(val)
	} else {
		s.LowValue = s.clamp(val)
	}
	if s.LowValue > s.HighValue {
		s.LowValue, s.HighValue = s.HighValue, s.LowValue
	}

	// A tap is a complete interaction, so it commits immediately.
	s.valuesChanged(lastLow, lastHigh)
	s.fireChangeEnded()
}

func (s *RangeSlider) HoverIn(e *event.HoverEvent) {
	s.hovered = true
	s.Refresh()
	s.fireHover(e)
}

func (s *RangeSlider) HoverMoved(e *event.HoverEvent) { s.fireHover(e) }

func (s *RangeSlider) HoverOut() {
	s.hovered = false
	s.Refresh()
	if s.OnHoverEnd != nil {
		s.OnHoverEnd()
	}
}

// valueAt maps a pointer position to the value under it, clamped to the range
// but not snapped to Step. Used for hover reporting, which wants the true
// pointer position.
func (s *RangeSlider) valueAt(e *event.PointEvent) float64 {
	val := s.Min + s.getRatio(e)*(s.Max-s.Min)
	return math.Max(s.Min, math.Min(s.Max, val))
}

func (s *RangeSlider) fireHover(e *event.HoverEvent) {
	if s.OnHover == nil {
		return
	}
	s.OnHover(s.valueAt(&e.PointEvent))
}

func (s *RangeSlider) MinSize() geo.Size {
	s.ExtendBaseWidget(s)
	return s.BaseWidget.MinSize()
}

func (s *RangeSlider) CreateRenderer() core.WidgetRenderer {
	s.ExtendBaseWidget(s)
	th := s.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	track := canvas.NewRectangle(th.Color(theme.ColorNameInputBackground, v))
	active := canvas.NewRectangle(th.Color(theme.ColorNamePrimary, v))
	lowThumb := &canvas.Circle{FillColor: th.Color(theme.ColorNameForeground, v)}
	highThumb := &canvas.Circle{FillColor: th.Color(theme.ColorNameForeground, v)}

	lowProxy := &rangeSliderThumb{slider: s, high: false}
	lowProxy.ExtendBaseWidget(lowProxy)
	highProxy := &rangeSliderThumb{slider: s, high: true}
	highProxy.ExtendBaseWidget(highProxy)

	objects := []core.CanvasObject{track, active, lowThumb, highThumb, lowProxy, highProxy}
	r := &rangeSliderRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer(objects),
		track:        track,
		active:       active,
		lowThumb:     lowThumb,
		highThumb:    highThumb,
		lowProxy:     lowProxy,
		highProxy:    highProxy,
		slider:       s,
	}
	r.Refresh()
	return r
}

func (s *RangeSlider) getRatio(e *event.PointEvent) float64 {
	th := s.Theme()
	inlineIconSize := th.Size(theme.SizeNameInlineIcon)
	innerPadding := th.Size(theme.SizeNameInnerPadding)
	endPad := float64(s.endOffset(inlineIconSize, innerPadding))

	if s.Orientation == Vertical {
		total := float64(s.Size().Height) - endPad*2
		if total <= 0 {
			return 0
		}
		return 1 - (float64(e.Position.Y)-endPad)/total
	}
	total := float64(s.Size().Width) - endPad*2
	if total <= 0 {
		return 0
	}
	return (float64(e.Position.X) - endPad) / total
}

func (s *RangeSlider) endOffset(iconSize, innerPad float32) float32 {
	return (s.buttonDiameter(iconSize) + innerPad) / 2
}

func (s *RangeSlider) buttonDiameter(iconSize float32) float32 {
	return iconSize / 2
}

func (s *RangeSlider) clamp(val float64) float64 {
	val = math.Max(s.Min, math.Min(s.Max, val))
	if s.Step > 0 {
		val = math.Round((val-s.Min)/s.Step)*s.Step + s.Min
		val = math.Max(s.Min, math.Min(s.Max, val))
	}
	return val
}

// --- Renderer ---

type rangeSliderRenderer struct {
	internalWidget.BaseRenderer
	track, active       *canvas.Rectangle
	lowThumb, highThumb *canvas.Circle
	// Accessibility-only proxies positioned over each thumb (#810).
	lowProxy, highProxy *rangeSliderThumb
	slider              *RangeSlider
}

func (r *rangeSliderRenderer) MinSize() geo.Size {
	th := r.slider.Theme()
	pad := th.Size(theme.SizeNameInnerPadding)
	tap := th.Size(theme.SizeNameInlineIcon)
	if r.slider.Orientation == Vertical {
		return geo.NewSize(tap+pad, tap+pad+minLongSide)
	}
	return geo.NewSize(tap+pad+minLongSide, tap+pad)
}

func (r *rangeSliderRenderer) Layout(size geo.Size) {
	th := r.slider.Theme()
	inputBorderSize := th.Size(theme.SizeNameInputBorder)
	trackWidth := inputBorderSize * 2
	inlineIconSize := th.Size(theme.SizeNameInlineIcon)
	innerPadding := th.Size(theme.SizeNameInnerPadding)
	diameter := r.slider.buttonDiameter(inlineIconSize)
	endPad := r.slider.endOffset(inlineIconSize, innerPadding)

	lowRatio := float32(0)
	highRatio := float32(1)
	if r.slider.Max > r.slider.Min {
		lowRatio = float32((r.slider.LowValue - r.slider.Min) / (r.slider.Max - r.slider.Min))
		highRatio = float32((r.slider.HighValue - r.slider.Min) / (r.slider.Max - r.slider.Min))
	}

	if r.slider.Orientation == Vertical {
		trackX := size.Width/2 - inputBorderSize
		r.track.Move(geo.NewPos(trackX, endPad))
		r.track.Resize(geo.NewSize(trackWidth, size.Height-endPad*2))

		trackLen := size.Height - endPad*2
		lowY := endPad + (1-lowRatio)*trackLen
		highY := endPad + (1-highRatio)*trackLen

		r.active.Move(geo.NewPos(trackX, highY))
		r.active.Resize(geo.NewSize(trackWidth, lowY-highY))

		r.lowThumb.Move(geo.NewPos(trackX-(diameter-trackWidth)/2, lowY-diameter/2))
		r.lowThumb.Resize(geo.NewSize(diameter, diameter))
		r.highThumb.Move(geo.NewPos(trackX-(diameter-trackWidth)/2, highY-diameter/2))
		r.highThumb.Resize(geo.NewSize(diameter, diameter))
	} else {
		trackY := size.Height/2 - inputBorderSize
		r.track.Move(geo.NewPos(endPad, trackY))
		r.track.Resize(geo.NewSize(size.Width-endPad*2, trackWidth))

		trackLen := size.Width - endPad*2
		lowX := endPad + lowRatio*trackLen
		highX := endPad + highRatio*trackLen

		r.active.Move(geo.NewPos(lowX, trackY))
		r.active.Resize(geo.NewSize(highX-lowX, trackWidth))

		r.lowThumb.Move(geo.NewPos(lowX-diameter/2, trackY-(diameter-trackWidth)/2))
		r.lowThumb.Resize(geo.NewSize(diameter, diameter))
		r.highThumb.Move(geo.NewPos(highX-diameter/2, trackY-(diameter-trackWidth)/2))
		r.highThumb.Resize(geo.NewSize(diameter, diameter))
	}

	// Keep the accessibility proxies aligned with their thumbs so a screen
	// reader's focus lands on each thumb (#810).
	r.lowProxy.Move(r.lowThumb.Position())
	r.lowProxy.Resize(r.lowThumb.Size())
	r.highProxy.Move(r.highThumb.Position())
	r.highProxy.Resize(r.highThumb.Size())
}

func (r *rangeSliderRenderer) Refresh() {
	th := r.slider.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	style := internaltheme.RenderStyleForWidget(r.slider)

	r.track.FillColor = th.Color(theme.ColorNameInputBackground, v)
	if style == internaltheme.RenderStyleModern {
		r.active.FillColor = th.Color(theme.ColorNamePrimary, v)
		radius := th.Size(theme.SizeNameInputBorder) * 2
		r.track.CornerRadius = radius
		r.active.CornerRadius = radius
	} else {
		r.active.FillColor = th.Color(theme.ColorNameForeground, v)
	}

	r.lowThumb.FillColor = th.Color(theme.ColorNameForeground, v)
	r.highThumb.FillColor = th.Color(theme.ColorNameForeground, v)

	r.Layout(r.slider.Size())
	canvas.Refresh(r.slider.super())
}
