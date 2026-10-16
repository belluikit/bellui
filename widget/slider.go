package widget

import (
	"fmt"
	"image/color"
	"math"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/data/binding"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internaltheme "github.com/belluikit/bellui/internal/theme"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

var (
	_ event.Draggable         = (*Slider)(nil)
	_ event.Focusable         = (*Slider)(nil)
	_ event.Hoverable         = (*Slider)(nil)
	_ event.Tappable          = (*Slider)(nil)
	_ core.Disableable        = (*Slider)(nil)
	_ core.Accessible         = (*Slider)(nil)
	_ core.AccessibleValuer   = (*Slider)(nil)
	_ core.AccessibleActioner = (*Slider)(nil)
)

// Slider is a widget that can slide between two fixed values.
type Slider struct {
	BaseWidget

	Value float64
	Min   float64
	Max   float64
	Step  float64

	Orientation Orientation
	OnChanged   func(float64) `json:"-"`

	// Since: 1.0
	OnChangeEnded func(float64) `json:"-"`

	// OnHover is called with the value under the pointer as it moves across the
	// slider, without changing the slider's own value. It is the seam a media
	// scrubber uses to preview the frame at a position before committing a seek.
	// Not called while the slider is disabled.
	//
	// The reported value is continuous — it is NOT snapped to Step, since a
	// preview wants the true pointer position and a consumer that wants the
	// snapped value can round it, while the reverse is not recoverable.
	//
	// Since: 1.0
	OnHover func(float64) `json:"-"`

	// OnHoverEnd is called when the pointer leaves the slider, so anything shown
	// in response to OnHover can be taken down again. It fires even when the
	// slider has been disabled mid-hover, so a consumer is never left displaying
	// a stale preview.
	//
	// Since: 1.0
	OnHoverEnd func() `json:"-"`

	// ShowValueIndicator displays a popup above the thumb with the current
	// value while dragging.
	ShowValueIndicator bool

	// ShowTickMarks draws small marks along the track at each Step interval.
	// Only visible when Step > 0.
	ShowTickMarks bool

	binder basicBinder
	// onChangedInternal is the binding write-back installed by Bind. It is a
	// private hook rather than the public OnChanged field so that binding and
	// an app's own handler compose instead of silently discarding each other.
	onChangedInternal func(float64)
	hovered           bool
	focused           bool
	disabled          bool // don't use DisableableWidget so we can put Since comments on funcs
	pendingChange     bool // true if value changed since last OnChangeEnded
	dragging          bool // true between Dragged and DragEnd
	valueIndicator    *PopUp
	// The indicator's label and content, kept so a drag updates them in place.
	valueIndicatorText    *canvas.Text
	valueIndicatorContent *bellui.Container
}

// NewSlider returns a basic slider.
func NewSlider(min, max float64) *Slider {
	slider := &Slider{
		Value:       0,
		Min:         min,
		Max:         max,
		Step:        1,
		Orientation: Horizontal,
	}
	slider.ExtendBaseWidget(slider)
	return slider
}

// NewSliderWithData returns a slider connected with the specified data source.
//
// Since: 1.0
func NewSliderWithData(min, max float64, data binding.Float) *Slider {
	slider := NewSlider(min, max)
	slider.Bind(data)

	return slider
}

// AccessibleRole returns [core.RoleSlider].
func (s *Slider) AccessibleRole() core.AccessibleRole { return core.RoleSlider }

// AccessibleLabel returns an empty string; sliders have no intrinsic text label.
func (s *Slider) AccessibleLabel() string { return "" }

// AccessibleValue returns a human-readable representation of the current value.
func (s *Slider) AccessibleValue() string {
	if s.Step >= 1 {
		return fmt.Sprintf("%d", int64(s.Value))
	}
	return fmt.Sprintf("%.1f", s.Value)
}

// AccessibleRange returns the minimum, maximum, and current slider values.
func (s *Slider) AccessibleRange() (min, max, current float64) {
	return s.Min, s.Max, s.Value
}

// AccessibleActions returns the "increment" and "decrement" actions.
func (s *Slider) AccessibleActions() []core.AccessibleAction {
	return []core.AccessibleAction{
		{Name: core.ActionIncrement, Description: "Increase the slider value"},
		{Name: core.ActionDecrement, Description: "Decrease the slider value"},
	}
}

// PerformAccessibleAction adjusts the slider value by one step.
func (s *Slider) PerformAccessibleAction(name string) {
	if s.Disabled() {
		return
	}
	switch name {
	case core.ActionIncrement:
		s.SetValue(s.Value + s.Step)
	case core.ActionDecrement:
		s.SetValue(s.Value - s.Step)
	}
}

// Bind connects the specified data source to this Slider.
// The current value will be displayed and any changes in the data will cause the widget to update.
// User interactions with this Slider will set the value into the data source.
//
// Binding does not claim [Slider.OnChanged]: the write-back rides a private
// hook, so an app handler set before or after Bind keeps working and the data
// source keeps tracking the widget either way.
//
// Since: 1.0
func (s *Slider) Bind(data binding.Float) {
	s.binder.SetCallback(s.updateFromData)
	s.binder.Bind(data)

	s.onChangedInternal = func(_ float64) {
		s.binder.CallWithData(s.writeData)
	}
}

// DragEnd is called when the drag ends.
func (s *Slider) DragEnd() {
	// Cleared unconditionally, and before the callback: a drag that never moved
	// the value past one Step fires no OnChangeEnded (fireChangeEnded returns
	// early without pendingChange), so a consumer inferring drag state from the
	// callback pair would latch on forever. Dragging() must not have that hole.
	s.dragging = false
	if !s.disabled {
		s.fireChangeEnded()
	}
	s.hideValueIndicator()
}

// Dragging reports whether the user is currently dragging the slider's thumb,
// between the first Dragged event and DragEnd.
//
// This is the authoritative signal a media scrubber needs in order to suppress
// its own periodic position updates while the user is scrubbing — otherwise the
// thumb jumps back under the pointer on every tick. Inferring it from
// OnChanged/OnChangeEnded instead is unreliable: a press and release that does
// not move the value fires neither callback.
//
// Since: 1.0
func (s *Slider) Dragging() bool {
	return s.dragging
}

// Dragged is called when a drag event occurs.
func (s *Slider) Dragged(e *event.DragEvent) {
	if s.disabled {
		return
	}
	s.dragging = true
	ratio := s.getRatio(&e.PointEvent)
	lastValue := s.Value

	s.updateValue(ratio)
	s.positionChanged(lastValue, s.Value)
	s.showValueIndicator()
}

// showValueIndicator shows the value popup above the thumb, or updates it.
// The popup is built once per drag and then updated in place: rebuilding it
// for every drag event replaced the overlay and restarted its fade-in each
// time, so the indicator trailed behind the thumb while dragging.
func (s *Slider) showValueIndicator() {
	if !s.ShowValueIndicator {
		return
	}
	c := bellui.CurrentApp().Driver().CanvasForObject(s)
	if c == nil {
		return
	}

	th := s.Theme()
	text := s.AccessibleValue()
	created := false
	if s.valueIndicator == nil || s.valueIndicator.Canvas != c {
		s.hideValueIndicator()
		v := bellui.CurrentApp().Settings().ThemeVariant()
		fgColor := th.Color(theme.ColorNameForeground, v)
		bgColor := th.Color(theme.ColorNameBackground, v)

		label := canvas.NewText(text, bgColor)
		label.TextSize = th.Size(theme.SizeNameCaptionText)
		bg := canvas.NewRectangle(fgColor)
		bg.CornerRadius = th.Size(theme.SizeNameInputRadius)
		content := &bellui.Container{
			Objects: []core.CanvasObject{bg, label},
			Layout:  &tooltipContentLayout{padding: th.Size(theme.SizeNamePadding)},
		}
		s.valueIndicator = newPopUpWithBgColor(content, c, fgColor)
		s.valueIndicatorText, s.valueIndicatorContent = label, content
		created = true
	} else if s.valueIndicatorText.Text != text {
		s.valueIndicatorText.Text = text
		s.valueIndicatorText.Refresh()
	}

	// Position above the thumb.
	absPos := bellui.CurrentApp().Driver().AbsolutePositionForObject(s)
	thumbPos := s.CursorPosition()
	indicatorSize := s.valueIndicatorContent.MinSize()
	pos := geo.NewPos(
		absPos.X+thumbPos.X-indicatorSize.Width/2,
		absPos.Y+thumbPos.Y-indicatorSize.Height-th.Size(theme.SizeNamePadding)*2,
	)
	if created {
		s.valueIndicator.ShowAtPosition(pos)
	} else {
		s.valueIndicator.Move(pos)
	}
}

func (s *Slider) hideValueIndicator() {
	if s.valueIndicator != nil {
		s.valueIndicator.Hide()
		s.valueIndicator = nil
		s.valueIndicatorText, s.valueIndicatorContent = nil, nil
	}
}

// CursorPosition returns the position of the slider thumb relative to the widget.
func (s *Slider) CursorPosition() geo.Position {
	th := s.Theme()
	inlineIconSize := th.Size(theme.SizeNameInlineIcon)
	innerPadding := th.Size(theme.SizeNameInnerPadding)
	inputBorderSize := th.Size(theme.SizeNameInputBorder)
	endPad := s.endOffset(inlineIconSize, innerPadding)
	trackWidth := inputBorderSize * 2

	size := s.Size()
	if s.Orientation == Vertical {
		ratio := float32(0)
		if s.Max > s.Min {
			ratio = float32((s.Value - s.Min) / (s.Max - s.Min))
		}
		y := size.Height - endPad - ratio*(size.Height-endPad*2)
		return geo.NewPos(size.Width/2, y)
	}
	ratio := float32(0)
	if s.Max > s.Min {
		ratio = float32((s.Value - s.Min) / (s.Max - s.Min))
	}
	x := endPad + ratio*(size.Width-endPad*2)
	return geo.NewPos(x, size.Height/2-trackWidth)
}

// Tapped is called when a pointer tapped event is captured.
//
// Since: 1.0
func (s *Slider) Tapped(e *event.PointEvent) {
	if s.disabled {
		return
	}

	if !s.focused {
		focusIfNotMobile(s.super())
	}

	ratio := s.getRatio(e)
	lastValue := s.Value

	s.updateValue(ratio)
	s.positionChanged(lastValue, s.Value)
	s.fireChangeEnded()
}

func (s *Slider) positionChanged(lastValue, currentValue float64) {
	if s.almostEqual(lastValue, currentValue) {
		return
	}

	s.Refresh()

	s.pendingChange = true
	// The bound data source is written first, so an OnChanged handler that reads
	// it does not see a stale value.
	if s.onChangedInternal != nil {
		s.onChangedInternal(s.Value)
	}
	if s.OnChanged != nil {
		s.OnChanged(s.Value)
	}
}

func (s *Slider) fireChangeEnded() {
	if !s.pendingChange {
		return
	}
	s.pendingChange = false
	if s.OnChangeEnded != nil {
		s.OnChangeEnded(s.Value)
	}
}

// FocusGained is called when this item gained the focus.
//
// Since: 1.0
func (s *Slider) FocusGained() {
	s.focused = true
	if !s.disabled {
		s.Refresh()
	}
}

// FocusLost is called when this item lost the focus.
//
// Since: 1.0
func (s *Slider) FocusLost() {
	s.focused = false
	if !s.disabled {
		s.Refresh()
	}
}

// HoverIn is called when a desktop pointer enters the widget.
//
// Since: 1.0
func (s *Slider) HoverIn(e *event.HoverEvent) {
	s.hovered = true
	if !s.disabled {
		s.Refresh()
		s.fireHover(e)
	}
}

// HoverMoved is called when a desktop pointer hovers over the widget.
//
// Since: 1.0
func (s *Slider) HoverMoved(e *event.HoverEvent) {
	if s.disabled {
		return
	}
	s.fireHover(e)
}

// HoverOut is called when a desktop pointer exits the widget
//
// Since: 1.0
func (s *Slider) HoverOut() {
	s.hovered = false
	if !s.disabled {
		s.Refresh()
	}
	// Fired even when disabled: a slider disabled mid-hover must not leave the
	// consumer showing a preview it can never take down.
	if s.OnHoverEnd != nil {
		s.OnHoverEnd()
	}
}

// TypedKey is called when this item receives a key event.
//
// Since: 1.0
func (s *Slider) TypedKey(key *event.KeyEvent) {
	if s.disabled {
		return
	}
	if s.Orientation == Vertical {
		switch key.Name {
		case event.KeyUp:
			s.SetValue(s.Value + s.Step)
		case event.KeyDown:
			s.SetValue(s.Value - s.Step)
		}
	} else {
		switch key.Name {
		case event.KeyLeft:
			s.SetValue(s.Value - s.Step)
		case event.KeyRight:
			s.SetValue(s.Value + s.Step)
		}
	}
}

// TypedRune is called when this item receives a char event.
//
// Since: 1.0
func (s *Slider) TypedRune(_ rune) {
}

func (s *Slider) buttonDiameter(inlineIconSize float32) float32 {
	return inlineIconSize - 4 // match radio icons
}

func (s *Slider) endOffset(inlineIconSize, innerPadding float32) float32 {
	return s.buttonDiameter(inlineIconSize)/2 + innerPadding - 1.5 // align with radio icons
}

func (s *Slider) getRatio(e *event.PointEvent) float64 {
	th := s.Theme()
	pad := s.endOffset(th.Size(theme.SizeNameInlineIcon), th.Size(theme.SizeNameInnerPadding))

	x := e.Position.X
	y := e.Position.Y
	size := s.Size()

	switch s.Orientation {
	case Vertical:
		if y > size.Height-pad {
			return 0.0
		} else if y < pad {
			return 1.0
		} else {
			return 1 - float64(y-pad)/float64(size.Height-pad*2)
		}
	case Horizontal:
		if x > size.Width-pad {
			return 1.0
		} else if x < pad {
			return 0.0
		} else {
			return float64(x-pad) / float64(size.Width-pad*2)
		}
	}
	return 0.0
}

func (s *Slider) clampValueToRange() {
	if s.Value >= s.Max {
		s.Value = s.Max
		return
	} else if s.Value <= s.Min {
		s.Value = s.Min
		return
	}

	if s.Step == 0 { // extended Slider may not have this set - assume value is not adjusted
		return
	}

	// only work with positive mods so the maths holds up
	value := s.Value
	step := s.Step
	invert := false
	if s.Value < 0 {
		invert = true
		value = -value
	}

	rem := math.Mod(value, step)
	if rem == 0 {
		return
	}
	min := value - rem
	if rem > step/2 {
		min += s.Step
	}

	if invert {
		s.Value = -min
	} else {
		s.Value = min
	}
}

func (s *Slider) updateValue(ratio float64) {
	s.Value = s.Min + ratio*(s.Max-s.Min)

	s.clampValueToRange()
}

// SetValue updates the value of the slider and clamps the value to be within the range.
// It is safe to call from any goroutine.
//
// The change is reported as though the user made it: both OnChanged and
// OnChangeEnded fire. That is deliberate — the arrow keys and the screen-reader
// increment/decrement actions route through here and must notify the app. For
// programmatic updates that should not look like user interaction, such as a
// media player advancing a position bar, use [Slider.SetValueSilently].
func (s *Slider) SetValue(value float64) {
	onMain(func() {
		if s.Value == value {
			return
		}
		lastValue := s.Value
		s.Value = value
		s.clampValueToRange()
		s.positionChanged(lastValue, s.Value)
		s.fireChangeEnded()
	})
}

// SetValueSilently updates the value and redraws, without firing OnChanged or
// OnChangeEnded. It is safe to call from any goroutine.
//
// This is the path for updates the user did not cause — a player advancing its
// position bar many times a second cannot use [Slider.SetValue], because every
// call would be indistinguishable from the user seeking.
//
// Because a bound data source is written on the same change notification the
// callbacks ride (see [Slider.Bind]), a silent update does NOT propagate to
// bound data. It is intended for display-only updates of an unbound slider; use
// [Slider.SetValue] to move a bound one.
//
// Since: 1.0
func (s *Slider) SetValueSilently(value float64) {
	onMain(func() {
		if s.Value == value {
			return
		}
		s.Value = value
		s.clampValueToRange()
		s.Refresh()
	})
}

// valueAt maps a pointer position to the value at that point, without snapping
// to Step. Used for hover reporting, which wants the true pointer position.
func (s *Slider) valueAt(e *event.PointEvent) float64 {
	return s.Min + s.getRatio(e)*(s.Max-s.Min)
}

func (s *Slider) fireHover(e *event.HoverEvent) {
	if s.OnHover == nil {
		return
	}
	s.OnHover(s.valueAt(&e.PointEvent))
}

// MinSize returns the size that this widget should not shrink below
func (s *Slider) MinSize() geo.Size {
	s.ExtendBaseWidget(s)
	return s.BaseWidget.MinSize()
}

// Disable disables the slider
//
// Since: 1.0
func (s *Slider) Disable() {
	if !s.disabled {
		defer s.Refresh()
	}
	s.disabled = true
}

// Enable enables the slider
//
// Since: 1.0
func (s *Slider) Enable() {
	if s.disabled {
		defer s.Refresh()
	}
	s.disabled = false
}

// Disabled returns true if the slider is currently disabled
//
// Since: 1.0
func (s *Slider) Disabled() bool {
	return s.disabled
}

// CreateRenderer links this widget to its renderer.
func (s *Slider) CreateRenderer() core.WidgetRenderer {
	s.ExtendBaseWidget(s)
	th := s.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	track := canvas.NewRectangle(th.Color(theme.ColorNameInputBackground, v))
	active := canvas.NewRectangle(th.Color(theme.ColorNameForeground, v))
	thumb := &canvas.Circle{FillColor: th.Color(theme.ColorNameForeground, v)}
	focusIndicator := &canvas.Circle{FillColor: color.Transparent}

	objects := []core.CanvasObject{track, active, thumb, focusIndicator}

	var ticks []*canvas.Circle
	if s.ShowTickMarks && s.Step > 0 && s.Max > s.Min {
		numTicks := int((s.Max-s.Min)/s.Step) + 1
		if numTicks > 100 {
			numTicks = 0 // too many ticks, skip
		}
		for i := 0; i < numTicks; i++ {
			tick := &canvas.Circle{FillColor: th.Color(theme.ColorNameInputBorder, v)}
			ticks = append(ticks, tick)
			objects = append(objects, tick)
		}
	}

	slide := &sliderRenderer{widget.NewBaseRenderer(objects), track, active, thumb, focusIndicator, ticks, s}
	slide.Refresh() // prepare for first draw
	return slide
}

func (s *Slider) almostEqual(a, b float64) bool {
	delta := math.Abs(a - b)
	return delta <= s.Step/2
}

func (s *Slider) updateFromData(data binding.DataItem) {
	if data == nil {
		return
	}
	floatSource, ok := data.(binding.Float)
	if !ok {
		return
	}

	val, err := floatSource.Get()
	if err != nil {
		bellui.LogError("Error getting current data value", err)
		return
	}
	s.SetValue(val) // if val != s.Value, this will call updateFromData again, but only once
}

func (s *Slider) writeData(data binding.DataItem) {
	if data == nil {
		return
	}
	floatTarget, ok := data.(binding.Float)
	if !ok {
		return
	}
	currentValue, err := floatTarget.Get()
	if err != nil {
		return
	}
	if s.Value != currentValue {
		err := floatTarget.Set(s.Value)
		if err != nil {
			bellui.LogError(fmt.Sprintf("Failed to set binding value to %f", s.Value), err)
		}
	}
}

// Unbind disconnects any configured data source from this Slider.
// The current value will remain at the last value of the data source.
//
// Only the write-back hook Bind installed is removed; [Slider.OnChanged]
// belongs to the app and is left alone.
//
// Since: 1.0
func (s *Slider) Unbind() {
	s.onChangedInternal = nil
	s.binder.Unbind()
}

const minLongSide = float32(34) // added to button diameter

type sliderRenderer struct {
	widget.BaseRenderer
	track          *canvas.Rectangle
	active         *canvas.Rectangle
	thumb          *canvas.Circle
	focusIndicator *canvas.Circle
	ticks          []*canvas.Circle
	slider         *Slider
}

// Refresh updates the widget state for drawing.
func (s *sliderRenderer) Refresh() {
	th := s.slider.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	style := internaltheme.RenderStyleForWidget(s.slider)

	s.track.FillColor = th.Color(theme.ColorNameInputBackground, v)
	if s.slider.disabled {
		s.thumb.FillColor = th.Color(theme.ColorNameDisabled, v)
		s.active.FillColor = s.thumb.FillColor
	} else {
		s.thumb.FillColor = th.Color(theme.ColorNameForeground, v)
		if style == internaltheme.RenderStyleModern {
			s.active.FillColor = th.Color(theme.ColorNamePrimary, v)
		} else {
			s.active.FillColor = s.thumb.FillColor
		}
	}

	if style == internaltheme.RenderStyleModern {
		radius := th.Size(theme.SizeNameInputBorder) * 2
		s.track.CornerRadius = radius
		s.active.CornerRadius = radius
	} else {
		s.track.CornerRadius = 0
		s.active.CornerRadius = 0
	}

	if s.slider.focused && !s.slider.disabled {
		s.focusIndicator.FillColor = th.Color(theme.ColorNameFocus, v)
	} else if s.slider.hovered && !s.slider.disabled {
		s.focusIndicator.FillColor = th.Color(theme.ColorNameHover, v)
	} else {
		s.focusIndicator.FillColor = color.Transparent
	}

	s.focusIndicator.Refresh()

	s.slider.clampValueToRange()
	s.Layout(s.slider.Size())
	canvas.Refresh(s.slider.super())
}

// Layout the components of the widget.
func (s *sliderRenderer) Layout(size geo.Size) {
	th := s.slider.Theme()

	inputBorderSize := th.Size(theme.SizeNameInputBorder)
	trackWidth := inputBorderSize * 2
	inlineIconSize := th.Size(theme.SizeNameInlineIcon)
	innerPadding := th.Size(theme.SizeNameInnerPadding)
	diameter := s.slider.buttonDiameter(inlineIconSize)
	endPad := s.slider.endOffset(inlineIconSize, innerPadding)

	var trackPos, activePos, thumbPos geo.Position
	var trackSize, activeSize geo.Size

	// some calculations are relative to trackSize, so we must update that first
	switch s.slider.Orientation {
	case Vertical:
		trackPos = geo.NewPos(size.Width/2-inputBorderSize, endPad)
		trackSize = geo.NewSize(trackWidth, size.Height-endPad*2)

	case Horizontal:
		trackPos = geo.NewPos(endPad, size.Height/2-inputBorderSize)
		trackSize = geo.NewSize(size.Width-endPad*2, trackWidth)
	}
	s.track.Move(trackPos)
	s.track.Resize(trackSize)

	// Pass the freshly-computed trackSize rather than re-reading s.track.Size(),
	// which lags a layout behind while the slider is being resized (pre-rebrand tracker #293).
	activeOffset := s.getOffset(trackSize, inlineIconSize, innerPadding)
	switch s.slider.Orientation {
	case Vertical:
		activePos = geo.NewPos(trackPos.X, activeOffset)
		activeSize = geo.NewSize(trackWidth, trackSize.Height-activeOffset+endPad)

		thumbPos = geo.NewPos(
			trackPos.X-(diameter-trackSize.Width)/2, activeOffset-(diameter/2),
		)
	case Horizontal:
		activePos = trackPos
		activeSize = geo.NewSize(activeOffset-endPad, trackWidth)

		thumbPos = geo.NewPos(
			activeOffset-(diameter/2), trackPos.Y-(diameter-trackSize.Height)/2,
		)
	}

	s.active.Move(activePos)
	s.active.Resize(activeSize)

	s.thumb.Move(thumbPos)
	s.thumb.Resize(geo.NewSize(diameter, diameter))

	focusIndicatorSize := geo.NewSquareSize(inlineIconSize + innerPadding)
	delta := (focusIndicatorSize.Width - diameter) / 2
	s.focusIndicator.Resize(focusIndicatorSize)
	s.focusIndicator.Move(thumbPos.SubtractXY(delta, delta))

	// Position tick marks.
	tickSize := float32(3)
	for i, tick := range s.ticks {
		ratio := float32(i) / float32(len(s.ticks)-1)
		tick.Resize(geo.NewSize(tickSize, tickSize))
		switch s.slider.Orientation {
		case Horizontal:
			x := endPad + ratio*(size.Width-endPad*2) - tickSize/2
			y := size.Height/2 - tickSize/2
			tick.Move(geo.NewPos(x, y))
		case Vertical:
			x := size.Width/2 - tickSize/2
			y := endPad + (1-ratio)*(size.Height-endPad*2) - tickSize/2
			tick.Move(geo.NewPos(x, y))
		}
	}
}

// MinSize calculates the minimum size of a widget.
func (s *sliderRenderer) MinSize() geo.Size {
	th := s.slider.Theme()
	pad := th.Size(theme.SizeNameInnerPadding)
	tap := th.Size(theme.SizeNameInlineIcon)
	dia := s.slider.buttonDiameter(tap)
	s1, s2 := minLongSide+dia, tap+pad*2

	switch s.slider.Orientation {
	case Vertical:
		return geo.NewSize(s2, s1)
	default:
		return geo.NewSize(s1, s2)
	}
}

func (s *sliderRenderer) getOffset(trackSize geo.Size, iconInlineSize, innerPadding float32) float32 {
	endPad := s.slider.endOffset(iconInlineSize, innerPadding)
	w := s.slider
	size := trackSize
	if w.Value == w.Min || w.Min == w.Max {
		switch w.Orientation {
		case Vertical:
			return size.Height + endPad
		case Horizontal:
			return endPad
		}
	}
	ratio := float32((w.Value - w.Min) / (w.Max - w.Min))

	switch w.Orientation {
	case Vertical:
		y := size.Height - ratio*size.Height + endPad
		return y
	case Horizontal:
		x := ratio*size.Width + endPad
		return x
	}

	return endPad
}
