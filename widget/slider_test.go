package widget

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/data/binding"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSliderWithData(t *testing.T) {
	val := binding.NewFloat()
	err := val.Set(4)
	assert.NoError(t, err)

	s := NewSliderWithData(0, 10, val)
	waitForBinding()
	assert.Equal(t, 4.0, s.Value)

	s.SetValue(2.0)
	f, err := val.Get()
	assert.NoError(t, err)
	assert.Equal(t, 2.0, f)
}

func TestSlider_Binding(t *testing.T) {
	s := NewSlider(0, 10)
	s.SetValue(2)
	assert.Equal(t, 2.0, s.Value)

	val := binding.NewFloat()
	s.Bind(val)
	waitForBinding()
	assert.Equal(t, 0.0, s.Value)

	err := val.Set(3)
	assert.NoError(t, err)
	waitForBinding()
	assert.Equal(t, 3.0, s.Value)

	s.SetValue(5)
	f, err := val.Get()
	assert.NoError(t, err)
	assert.Equal(t, 5.0, f)

	s.Unbind()
	waitForBinding()
	assert.Equal(t, 5.0, s.Value)
}

// Bind used to implement its write-back by assigning the public OnChanged
// field, so an app handler set before Bind was silently discarded (#47).
func TestSlider_Bind_KeepsOnChangedSetBefore(t *testing.T) {
	s := NewSlider(0, 10)
	s.Step = 1

	handled := 0
	s.OnChanged = func(float64) { handled++ }

	val := binding.NewFloat()
	s.Bind(val)
	waitForBinding()

	s.SetValue(4)
	waitForBinding()

	assert.Equal(t, 1, handled, "the handler set before Bind still fires")
	f, err := val.Get()
	assert.NoError(t, err)
	assert.Equal(t, 4.0, f, "and the binding write-back still lands")
}

// The other direction, and the likelier one to hit: NewSliderWithData binds for
// you, so nothing signals that OnChanged is already spoken for. Assigning it
// used to discard the write-back and the data source quietly stopped tracking.
func TestSlider_Bind_KeepsWriteBackWhenOnChangedSetAfter(t *testing.T) {
	val := binding.NewFloat()
	s := NewSliderWithData(0, 10, val)
	waitForBinding()
	s.Step = 1

	handled := 0
	s.OnChanged = func(float64) { handled++ }

	s.SetValue(6)
	waitForBinding()

	assert.Equal(t, 1, handled)
	f, err := val.Get()
	assert.NoError(t, err)
	assert.Equal(t, 6.0, f, "the data source keeps tracking the widget")
}

func TestSlider_Unbind_KeepsOnChanged(t *testing.T) {
	s := NewSlider(0, 10)
	s.Step = 1

	handled := 0
	s.OnChanged = func(float64) { handled++ }

	val := binding.NewFloat()
	s.Bind(val)
	waitForBinding()

	s.Unbind()
	waitForBinding()

	s.SetValue(3)
	assert.Equal(t, 1, handled, "Unbind removes only the hook Bind installed")
	f, err := val.Get()
	assert.NoError(t, err)
	assert.Equal(t, 0.0, f, "but the data source is no longer written")
}

func TestSlider_Clamp(t *testing.T) {
	s := &Slider{Min: 0, Max: 1, Step: 0.001}
	s.Value = 0.959

	s.clampValueToRange()
	assert.InEpsilon(t, 0.959, s.Value, 0.00001)

	s.Min = -1
	s.Max = 0
	s.Value = -0.959

	s.clampValueToRange()
	assert.InEpsilon(t, -0.959, s.Value, 0.00001)
}

func TestSlider_HorizontalLayout(t *testing.T) {
	app := test.NewTempApp(t)
	app.Settings().SetTheme(test.Theme())

	slider := NewSlider(0, 1)
	slider.Resize(geo.NewSize(100, 10))

	render := test.TempWidgetRenderer(t, slider).(*sliderRenderer)
	wSize := render.slider.Size()
	tSize := render.track.Size()
	aSize := render.active.Size()

	assert.Greater(t, wSize.Width, wSize.Height)

	endOffset := slider.endOffset(theme.IconInlineSize(), theme.InnerPadding())
	assert.Equal(t, wSize.Width-endOffset*2, tSize.Width)
	assert.Equal(t, theme.InputBorderSize()*2, tSize.Height)

	assert.Greater(t, wSize.Width, aSize.Width)
	assert.Equal(t, theme.InputBorderSize()*2, aSize.Height)

	w := test.NewWindow(slider)
	defer w.Close()
	w.Resize(geo.NewSize(220, 50))

	test.AssertRendersToMarkup(t, "slider/horizontal.xml", w.Canvas())
}

func TestSlider_MinSize(t *testing.T) {
	min := NewSlider(0, 10).MinSize()

	assert.Equal(t, theme.IconInlineSize()+theme.InnerPadding()*2, min.Height)
}

func TestSlider_OutOfRange(t *testing.T) {
	slider := NewSlider(2, 5)
	slider.Resize(geo.NewSize(100, 10))

	assert.Equal(t, float64(2), slider.Value)
}

func TestSlider_VerticalLayout(t *testing.T) {
	app := test.NewTempApp(t)
	app.Settings().SetTheme(test.Theme())

	slider := NewSlider(0, 1)
	slider.Orientation = Vertical
	slider.Resize(geo.NewSize(10, 100))

	render := test.TempWidgetRenderer(t, slider).(*sliderRenderer)
	wSize := render.slider.Size()
	tSize := render.track.Size()
	aSize := render.active.Size()

	assert.Greater(t, wSize.Height, wSize.Width)

	endOffset := slider.endOffset(theme.IconInlineSize(), theme.InnerPadding())
	assert.Equal(t, wSize.Height-endOffset*2, tSize.Height)
	assert.Equal(t, theme.InputBorderSize()*2, tSize.Width)

	assert.Greater(t, wSize.Height, aSize.Height)
	assert.Equal(t, theme.InputBorderSize()*2, aSize.Width)

	w := test.NewWindow(slider)
	defer w.Close()
	w.Resize(geo.NewSize(50, 220))

	test.AssertRendersToMarkup(t, "slider/vertical.xml", w.Canvas())
}

func TestSlider_OnChanged(t *testing.T) {
	slider := NewSlider(0, 2)
	slider.Resize(slider.MinSize())
	assert.Empty(t, slider.OnChanged)

	changes := 0

	slider.OnChanged = func(_ float64) {
		changes++
	}

	assert.Equal(t, 0, changes)

	slider.SetValue(0.5)
	assert.Equal(t, 0, changes)

	drag := &event.DragEvent{}
	drag.PointEvent.Position = geo.NewPos(25, 2)
	slider.Dragged(drag)
	assert.Equal(t, 1, changes)

	drag.PointEvent.Position = geo.NewPos(25, 2)
	slider.Dragged(drag)
	assert.Equal(t, 1, changes)

	drag.PointEvent.Position = geo.NewPos(50, 2)
	slider.Dragged(drag)
	assert.Equal(t, 2, changes)

	tap := &event.PointEvent{}
	tap.Position = geo.NewPos(25, 2)
	slider.Tapped(tap)
	assert.Equal(t, 3, changes)

	tap.Position = geo.NewPos(25, 2)
	slider.Tapped(tap)
	assert.Equal(t, 3, changes)

	tap.Position = geo.NewPos(50, 2)
	slider.Tapped(tap)
	assert.Equal(t, 4, changes)

	slider.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	assert.Equal(t, 5, changes)
}

func TestSlider_OnChangeEnded(t *testing.T) {
	slider := NewSlider(0, 2)
	slider.Resize(slider.MinSize())
	assert.Empty(t, slider.OnChangeEnded)

	changes := 0

	slider.OnChangeEnded = func(_ float64) {
		changes++
	}

	assert.Equal(t, 0, changes)

	slider.SetValue(0.5)
	assert.Equal(t, 0, changes)

	drag := &event.DragEvent{}
	drag.PointEvent.Position = geo.NewPos(25, 2)
	slider.Dragged(drag)
	assert.Equal(t, 0, changes)

	drag.PointEvent.Position = geo.NewPos(25, 2)
	slider.Dragged(drag)
	assert.Equal(t, 0, changes)

	drag.PointEvent.Position = geo.NewPos(50, 2)
	slider.Dragged(drag)
	slider.DragEnd()
	assert.Equal(t, 1, changes)

	tap := &event.PointEvent{}
	tap.Position = geo.NewPos(25, 2)
	slider.Tapped(tap)
	assert.Equal(t, 2, changes)

	tap.Position = geo.NewPos(25, 2)
	slider.Tapped(tap)
	assert.Equal(t, 2, changes)

	tap.Position = geo.NewPos(50, 2)
	slider.Tapped(tap)
	assert.Equal(t, 3, changes)

	slider.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	assert.Equal(t, 4, changes)
}

func TestSlider_OnChanged_Float(t *testing.T) {
	slider := NewSlider(0, 1)
	slider.Step = 0.5
	slider.Resize(slider.MinSize())
	assert.Empty(t, slider.OnChanged)

	changes := 0

	slider.OnChanged = func(_ float64) {
		changes++
	}

	assert.Equal(t, 0, changes)

	slider.SetValue(0.15)
	assert.Equal(t, 0, changes)

	drag := &event.DragEvent{}
	drag.PointEvent.Position = geo.NewPos(25, 2)
	slider.Dragged(drag)
	assert.Equal(t, 1, changes)

	drag.PointEvent.Position = geo.NewPos(25, 2)
	slider.Dragged(drag)
	assert.Equal(t, 1, changes)

	drag.PointEvent.Position = geo.NewPos(50, 2)
	slider.Dragged(drag)
	assert.Equal(t, 2, changes)

	tap := &event.PointEvent{}
	tap.Position = geo.NewPos(25, 2)
	slider.Tapped(tap)
	assert.Equal(t, 3, changes)

	tap.Position = geo.NewPos(25, 2)
	slider.Tapped(tap)
	assert.Equal(t, 3, changes)

	tap.Position = geo.NewPos(50, 2)
	slider.Tapped(tap)
	assert.Equal(t, 4, changes)

	slider.SetValue(0.9)
	assert.Equal(t, 4, changes)
}

func TestSlider_SetValue(t *testing.T) {
	slider := NewSlider(0, 2)
	assert.Equal(t, 0.0, slider.Value)

	slider.SetValue(1.0)
	assert.Equal(t, 1.0, slider.Value)

	slider.SetValue(2.0)
	assert.Equal(t, 2.0, slider.Value)
}

func TestSlider_FocusDesktop(t *testing.T) {
	if bellui.CurrentDevice().IsMobile() {
		return
	}
	slider := NewSlider(0, 10)
	win := test.NewTempWindow(t, slider)
	test.Tap(slider)

	assert.Equal(t, win.Canvas().Focused(), slider)
	assert.True(t, slider.focused)
}

func TestSlider_Focus(t *testing.T) {
	slider := NewSlider(0, 5)
	slider.FocusGained()
	assert.True(t, slider.focused)

	slider.FocusLost()
	assert.False(t, slider.focused)

	slider.HoverIn(nil)
	assert.True(t, slider.hovered)

	slider.HoverOut()
	assert.False(t, slider.hovered)

	left := &event.KeyEvent{Name: event.KeyLeft}
	right := &event.KeyEvent{Name: event.KeyRight}

	for i := 1; i <= int(slider.Max); i++ {
		slider.TypedKey(right)
		assert.Equal(t, float64(i), slider.Value)
	}

	slider.TypedKey(right)
	assert.Equal(t, slider.Max, slider.Value)

	for i := 4; i >= int(slider.Min); i-- {
		slider.TypedKey(left)
		assert.Equal(t, float64(i), slider.Value)
	}

	slider.TypedKey(left)
	assert.Equal(t, slider.Min, slider.Value)

	slider.Orientation = Vertical
	up := &event.KeyEvent{Name: event.KeyUp}
	down := &event.KeyEvent{Name: event.KeyDown}

	slider.TypedKey(up)
	assert.Equal(t, slider.Min+1, slider.Value)

	slider.TypedKey(down)
	assert.Equal(t, slider.Min, slider.Value)
}

func TestSlider_Disabled(t *testing.T) {
	slider := NewSlider(0, 5)
	slider.Disable()

	changes := 0
	slider.OnChanged = func(_ float64) {
		changes++
	}

	tap := &event.PointEvent{}
	tap.Position = geo.NewPos(30, 2)
	slider.Tapped(tap)
	assert.Equal(t, 0, changes)

	drag := &event.DragEvent{}
	drag.PointEvent.Position = geo.NewPos(25, 2)
	slider.Dragged(drag)
	assert.Equal(t, 0, changes)

	slider.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, 0, changes)

	slider.Enable()

	slider.Dragged(drag)
	assert.Equal(t, 1, changes)
}

func TestSlider_Dragging(t *testing.T) {
	slider := NewSlider(0, 2)
	slider.Resize(slider.MinSize())
	assert.False(t, slider.Dragging(), "a fresh slider is not being dragged")

	drag := &event.DragEvent{}
	drag.PointEvent.Position = geo.NewPos(25, 2)
	slider.Dragged(drag)
	assert.True(t, slider.Dragging())

	drag.PointEvent.Position = geo.NewPos(50, 2)
	slider.Dragged(drag)
	assert.True(t, slider.Dragging(), "still dragging across moves")

	slider.DragEnd()
	assert.False(t, slider.Dragging())
}

// The failure mode Dragging() exists to close: a press and release that never
// moves the value past one Step fires neither OnChanged nor OnChangeEnded, so a
// consumer inferring drag state from that pair would latch on forever.
func TestSlider_Dragging_ClearsWhenNoCallbacksFire(t *testing.T) {
	slider := NewSlider(0, 2)
	slider.Resize(slider.MinSize())

	changed, ended := 0, 0
	slider.OnChanged = func(float64) { changed++ }
	slider.OnChangeEnded = func(float64) { ended++ }

	drag := &event.DragEvent{}
	drag.PointEvent.Position = geo.NewPos(0, 2) // value stays at Min
	slider.Dragged(drag)
	assert.True(t, slider.Dragging())
	slider.DragEnd()

	assert.Equal(t, 0, changed, "no value change, so no OnChanged")
	assert.Equal(t, 0, ended, "and therefore no OnChangeEnded either")
	assert.False(t, slider.Dragging(), "drag state must still have cleared")
}

func TestSlider_Dragging_IgnoredWhenDisabled(t *testing.T) {
	slider := NewSlider(0, 2)
	slider.Resize(slider.MinSize())
	slider.Disable()

	drag := &event.DragEvent{}
	drag.PointEvent.Position = geo.NewPos(25, 2)
	slider.Dragged(drag)
	assert.False(t, slider.Dragging(), "a disabled slider cannot be dragged")
}

func TestSlider_SetValueSilently(t *testing.T) {
	slider := NewSlider(0, 10)
	slider.Step = 1

	changed, ended := 0, 0
	slider.OnChanged = func(float64) { changed++ }
	slider.OnChangeEnded = func(float64) { ended++ }

	slider.SetValueSilently(4)
	assert.Equal(t, 4.0, slider.Value, "the value still updates")
	assert.Equal(t, 0, changed, "OnChanged must not fire for a programmatic set")
	assert.Equal(t, 0, ended, "nor OnChangeEnded")

	// Contrast: SetValue is the user-intent path and still reports.
	slider.SetValue(6)
	assert.Equal(t, 6.0, slider.Value)
	assert.Equal(t, 1, changed)
	assert.Equal(t, 1, ended)
}

func TestSlider_SetValueSilently_Clamps(t *testing.T) {
	slider := NewSlider(0, 10)
	slider.Step = 1

	slider.SetValueSilently(99)
	assert.Equal(t, 10.0, slider.Value)

	slider.SetValueSilently(-99)
	assert.Equal(t, 0.0, slider.Value)
}

func TestSlider_OnHover(t *testing.T) {
	slider := NewSlider(0, 100)
	slider.Step = 1
	slider.Resize(geo.NewSize(200, 20))

	var values []float64
	slider.OnHover = func(v float64) { values = append(values, v) }
	ends := 0
	slider.OnHoverEnd = func() { ends++ }

	hover := func(x float32) *event.HoverEvent {
		e := &event.HoverEvent{}
		e.PointEvent.Position = geo.NewPos(x, 10)
		return e
	}

	slider.HoverIn(hover(20))
	slider.HoverMoved(hover(150))
	require.Len(t, values, 2, "entering and moving both report a position")
	assert.Less(t, values[0], values[1], "further right is a larger value")
	assert.GreaterOrEqual(t, values[0], 0.0)
	assert.LessOrEqual(t, values[1], 100.0)

	assert.Equal(t, 0.0, slider.Value, "hovering must not change the value")

	slider.HoverOut()
	assert.Equal(t, 1, ends)
}

func TestSlider_OnHover_NotWhileDisabled(t *testing.T) {
	slider := NewSlider(0, 100)
	slider.Resize(geo.NewSize(200, 20))
	slider.Disable()

	hovers, ends := 0, 0
	slider.OnHover = func(float64) { hovers++ }
	slider.OnHoverEnd = func() { ends++ }

	e := &event.HoverEvent{}
	e.PointEvent.Position = geo.NewPos(100, 10)
	slider.HoverIn(e)
	slider.HoverMoved(e)
	assert.Equal(t, 0, hovers, "a disabled slider reports no hover position")

	// HoverEnd still fires, so a slider disabled mid-hover cannot leave a
	// consumer showing a preview it can never take down.
	slider.HoverOut()
	assert.Equal(t, 1, ends)
}

// The screen-reader increment/decrement actions are user interaction and must
// notify the app — previously asserted only to move the value, not to report it.
func TestSlider_PerformAccessibleAction_ReportsChange(t *testing.T) {
	slider := NewSlider(0, 10)
	slider.Step = 1
	slider.SetValueSilently(5)

	changed, ended := 0, 0
	slider.OnChanged = func(float64) { changed++ }
	slider.OnChangeEnded = func(float64) { ended++ }

	slider.PerformAccessibleAction(core.ActionIncrement)
	assert.Equal(t, 6.0, slider.Value)
	assert.Equal(t, 1, changed)
	assert.Equal(t, 1, ended)

	slider.PerformAccessibleAction(core.ActionDecrement)
	assert.Equal(t, 5.0, slider.Value)
	assert.Equal(t, 2, changed)
	assert.Equal(t, 2, ended)
}

// Arrow keys are user interaction too; OnChanged was covered, OnChangeEnded was not.
func TestSlider_TypedKey_ReportsChangeEnded(t *testing.T) {
	slider := NewSlider(0, 10)
	slider.Step = 1
	slider.SetValueSilently(5)

	ended := 0
	slider.OnChangeEnded = func(float64) { ended++ }

	slider.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, 6.0, slider.Value)
	assert.Equal(t, 1, ended)
}
