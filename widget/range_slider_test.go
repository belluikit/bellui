package widget_test

import (
	"math"
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sizedRangeSlider shows the slider in a temp window and resizes it so that
// interaction-position maths have a real, sizeable track to work against.
func sizedRangeSlider(t *testing.T, s *widget.RangeSlider, w, h float32) {
	win := test.NewTempWindow(t, s)
	win.Resize(geo.NewSize(w, h))
}

func tapAt(x, y float32) *event.PointEvent {
	return &event.PointEvent{Position: geo.NewPos(x, y)}
}

func TestRangeSlider_New(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	assert.NotNil(t, s)
	assert.Equal(t, 0.0, s.LowValue)
	assert.Equal(t, 100.0, s.HighValue)
	assert.Equal(t, 0.0, s.Min)
	assert.Equal(t, 100.0, s.Max)
}

func TestRangeSlider_SetValues(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.SetValues(25, 75)
	assert.Equal(t, 25.0, s.LowValue)
	assert.Equal(t, 75.0, s.HighValue)
}

func TestRangeSlider_SetValues_Swap(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.SetValues(80, 20)
	assert.Equal(t, 20.0, s.LowValue)
	assert.Equal(t, 80.0, s.HighValue)
}

func TestRangeSlider_SetValues_Clamp(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(10, 90)
	s.SetValues(-5, 200)
	assert.Equal(t, 10.0, s.LowValue)
	assert.Equal(t, 90.0, s.HighValue)
}

func TestRangeSlider_MinSize(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	min := s.MinSize()
	assert.Greater(t, min.Width, float32(0))
	assert.Greater(t, min.Height, float32(0))
}

func TestRangeSlider_Tapped_FiresOnChanged(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.SetValues(40, 60)
	sizedRangeSlider(t, s, 400, 60)

	var gotLow, gotHigh float64
	var calls int
	s.OnChanged = func(low, high float64) {
		gotLow, gotHigh = low, high
		calls++
	}

	// Tapping the exact horizontal centre maps to ratio 0.5 → value 50,
	// regardless of end padding, so the high thumb (nearer to 60 by the tie
	// rule) moves there.
	s.Tapped(tapAt(s.Size().Width/2, s.Size().Height/2))

	assert.Equal(t, 1, calls, "tap fires OnChanged once")
	assert.Equal(t, s.LowValue, gotLow)
	assert.Equal(t, s.HighValue, gotHigh)
	assert.Equal(t, 50.0, s.HighValue)
	assert.Equal(t, 40.0, s.LowValue, "the far thumb is left untouched")
}

func TestRangeSlider_Tapped_Proximity(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.SetValues(40, 60)
	sizedRangeSlider(t, s, 400, 60)
	mid := s.Size().Height / 2

	// Far right is nearest the high thumb by value.
	s.Tapped(tapAt(s.Size().Width, mid))
	assert.Equal(t, 100.0, s.HighValue)
	assert.Equal(t, 40.0, s.LowValue)

	// Far left is nearest the low thumb by value.
	s.Tapped(tapAt(0, mid))
	assert.Equal(t, 0.0, s.LowValue)
	assert.Equal(t, 100.0, s.HighValue)
}

func TestRangeSlider_Tapped_StepQuantization(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.Step = 10
	s.SetValues(0, 100)
	sizedRangeSlider(t, s, 400, 60)

	// Centre tap → value 50; nearer the high thumb by the tie rule.
	s.Tapped(tapAt(s.Size().Width/2, s.Size().Height/2))
	assert.Equal(t, 50.0, s.HighValue)
	assert.Zero(t, math.Mod(s.HighValue, s.Step), "value snaps to a Step multiple")
}

func TestRangeSlider_Drag_CrossPrevention(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.SetValues(20, 50)
	sizedRangeSlider(t, s, 400, 60)
	w, mid := s.Size().Width, s.Size().Height/2

	drag := func(x float32) {
		s.Dragged(&event.DragEvent{PointEvent: *tapAt(x, mid)})
	}
	// Grab near the low thumb (≈ value 25), then drag well past the high thumb.
	drag(w * 0.25)
	drag(w * 0.95)
	s.DragEnd()

	assert.Equal(t, 50.0, s.LowValue, "low thumb is clamped at the high thumb, not crossed over")
	assert.Equal(t, 50.0, s.HighValue)
}

func TestRangeSlider_Dragging(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.SetValues(20, 50)
	sizedRangeSlider(t, s, 400, 60)
	w, mid := s.Size().Width, s.Size().Height/2

	assert.False(t, s.Dragging(), "a fresh slider is not being dragged")

	s.Dragged(&event.DragEvent{PointEvent: *tapAt(w*0.25, mid)})
	assert.True(t, s.Dragging())
	assert.False(t, s.DraggingHigh(), "grabbed nearest the low thumb")

	s.Dragged(&event.DragEvent{PointEvent: *tapAt(w*0.3, mid)})
	assert.True(t, s.Dragging(), "still dragging across moves")

	s.DragEnd()
	assert.False(t, s.Dragging())
}

func TestRangeSlider_DraggingHigh(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.SetValues(20, 50)
	sizedRangeSlider(t, s, 400, 60)
	w, mid := s.Size().Width, s.Size().Height/2

	// Grabbing near value 90 is nearest the high thumb.
	s.Dragged(&event.DragEvent{PointEvent: *tapAt(w*0.9, mid)})
	assert.True(t, s.Dragging())
	assert.True(t, s.DraggingHigh())
	assert.Equal(t, 20.0, s.LowValue, "the far thumb is left untouched")
	s.DragEnd()
}

// The failure mode Dragging() exists to close: a press and release that never
// moves a thumb fires neither OnChanged nor OnChangeEnded, so a consumer
// inferring drag state from that pair would latch on forever.
func TestRangeSlider_Dragging_ClearsWhenNoCallbacksFire(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.SetValues(0, 100)
	sizedRangeSlider(t, s, 400, 60)

	changed, ended := 0, 0
	s.OnChanged = func(float64, float64) { changed++ }
	s.OnChangeEnded = func(float64, float64) { ended++ }

	// Grab the low thumb where it already sits, at the far left.
	s.Dragged(&event.DragEvent{PointEvent: *tapAt(0, s.Size().Height/2)})
	assert.True(t, s.Dragging())
	s.DragEnd()

	assert.Equal(t, 0, changed, "no thumb moved, so no OnChanged")
	assert.Equal(t, 0, ended, "and therefore no OnChangeEnded either")
	assert.False(t, s.Dragging(), "drag state must still have cleared")
}

func TestRangeSlider_OnChangeEnded_Drag(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.SetValues(20, 50)
	sizedRangeSlider(t, s, 400, 60)
	w, mid := s.Size().Width, s.Size().Height/2

	changed, ended := 0, 0
	var endLow, endHigh float64
	s.OnChanged = func(float64, float64) { changed++ }
	s.OnChangeEnded = func(low, high float64) {
		ended++
		endLow, endHigh = low, high
	}

	s.Dragged(&event.DragEvent{PointEvent: *tapAt(w*0.25, mid)})
	s.Dragged(&event.DragEvent{PointEvent: *tapAt(w*0.35, mid)})
	assert.Equal(t, 2, changed, "each move reports in flight")
	assert.Equal(t, 0, ended, "but nothing is committed mid-drag")

	s.DragEnd()
	assert.Equal(t, 1, ended, "the drag commits exactly once")
	assert.Equal(t, s.LowValue, endLow)
	assert.Equal(t, s.HighValue, endHigh)

	// A second DragEnd with nothing pending must not re-report.
	s.DragEnd()
	assert.Equal(t, 1, ended)
}

func TestRangeSlider_OnChangeEnded_Tap(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.SetValues(40, 60)
	sizedRangeSlider(t, s, 400, 60)

	changed, ended := 0, 0
	s.OnChanged = func(float64, float64) { changed++ }
	s.OnChangeEnded = func(float64, float64) { ended++ }

	// A tap is a complete interaction, so it reports both at once.
	s.Tapped(tapAt(s.Size().Width/2, s.Size().Height/2))
	assert.Equal(t, 1, changed)
	assert.Equal(t, 1, ended)

	// Tapping the same spot again moves nothing and reports nothing.
	s.Tapped(tapAt(s.Size().Width/2, s.Size().Height/2))
	assert.Equal(t, 1, changed)
	assert.Equal(t, 1, ended)
}

// SetValues follows the same rule as Slider.SetValue: a programmatic set is
// reported as though the user made it.
func TestRangeSlider_SetValues_Reports(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)

	changed, ended := 0, 0
	var gotLow, gotHigh float64
	s.OnChanged = func(low, high float64) {
		changed++
		gotLow, gotHigh = low, high
	}
	s.OnChangeEnded = func(float64, float64) { ended++ }

	s.SetValues(25, 75)
	assert.Equal(t, 1, changed)
	assert.Equal(t, 1, ended)
	assert.Equal(t, 25.0, gotLow)
	assert.Equal(t, 75.0, gotHigh)

	// Setting the values it already holds is not a change.
	s.SetValues(25, 75)
	assert.Equal(t, 1, changed)
	assert.Equal(t, 1, ended)
}

func TestRangeSlider_SetValuesSilently(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)

	changed, ended := 0, 0
	s.OnChanged = func(float64, float64) { changed++ }
	s.OnChangeEnded = func(float64, float64) { ended++ }

	s.SetValuesSilently(25, 75)
	assert.Equal(t, 25.0, s.LowValue, "the values still update")
	assert.Equal(t, 75.0, s.HighValue)
	assert.Equal(t, 0, changed, "OnChanged must not fire for a silent set")
	assert.Equal(t, 0, ended, "nor OnChangeEnded")

	// It clamps and orders like SetValues.
	s.SetValuesSilently(200, -5)
	assert.Equal(t, 0.0, s.LowValue)
	assert.Equal(t, 100.0, s.HighValue)
	assert.Equal(t, 0, changed)
}

func TestRangeSlider_SetValues_SnapsToStep(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.Step = 10

	// A programmatic set lands on the same grid a drag would, so a consumer
	// acting on the reported values sees one value space, not two.
	var gotLow, gotHigh float64
	s.OnChanged = func(low, high float64) { gotLow, gotHigh = low, high }

	s.SetValues(23, 77)
	assert.Equal(t, 20.0, s.LowValue)
	assert.Equal(t, 80.0, s.HighValue)
	assert.Equal(t, s.LowValue, gotLow)
	assert.Equal(t, s.HighValue, gotHigh)
}

func TestRangeSlider_OnHover(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.Step = 10
	s.SetValues(20, 80)
	sizedRangeSlider(t, s, 400, 60)
	mid := s.Size().Height / 2

	var values []float64
	s.OnHover = func(v float64) { values = append(values, v) }
	ends := 0
	s.OnHoverEnd = func() { ends++ }

	hover := func(x float32) *event.HoverEvent {
		return &event.HoverEvent{PointEvent: *tapAt(x, mid)}
	}

	s.HoverIn(hover(s.Size().Width * 0.25))
	s.HoverMoved(hover(s.Size().Width * 0.75))
	require.Len(t, values, 2, "entering and moving both report a position")
	assert.Less(t, values[0], values[1], "further right is a larger value")
	assert.GreaterOrEqual(t, values[0], 0.0)
	assert.LessOrEqual(t, values[1], 100.0)

	// Continuous, not snapped to Step — a preview wants the true pointer
	// position, and rounding is recoverable while un-rounding is not.
	assert.NotZero(t, math.Mod(values[0], s.Step), "the reported value is not quantized")

	assert.Equal(t, 20.0, s.LowValue, "hovering must not move a thumb")
	assert.Equal(t, 80.0, s.HighValue)

	s.HoverOut()
	assert.Equal(t, 1, ends)
}

func TestRangeSlider_OnHover_ClampedToRange(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	sizedRangeSlider(t, s, 400, 60)
	mid := s.Size().Height / 2

	var values []float64
	s.OnHover = func(v float64) { values = append(values, v) }

	// The end padding sits outside the track proper; the reported value must
	// still be a value this slider can hold.
	s.HoverMoved(&event.HoverEvent{PointEvent: *tapAt(0, mid)})
	s.HoverMoved(&event.HoverEvent{PointEvent: *tapAt(s.Size().Width, mid)})
	require.Len(t, values, 2)
	assert.Equal(t, 0.0, values[0])
	assert.Equal(t, 100.0, values[1])
}

func TestRangeSlider_Vertical_Inverted(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.Orientation = widget.Vertical
	s.SetValues(20, 80)
	sizedRangeSlider(t, s, 60, 400)

	// In vertical orientation the top of the track is the maximum, so a tap at
	// the top edge drives the high thumb to Max.
	s.Tapped(tapAt(s.Size().Width/2, 0))
	assert.Equal(t, 100.0, s.HighValue)
	assert.Equal(t, 20.0, s.LowValue)
}

// --- Disabled state (#51) ---

func TestRangeSlider_Disable(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	assert.False(t, s.Disabled())

	s.Disable()
	assert.True(t, s.Disabled())

	s.Enable()
	assert.False(t, s.Disabled())
}

func TestRangeSlider_Disabled_IgnoresPointer(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.SetValuesSilently(20, 80)
	sizedRangeSlider(t, s, 400, 60)
	mid := s.Size().Height / 2

	changed, ended := 0, 0
	s.OnChanged = func(float64, float64) { changed++ }
	s.OnChangeEnded = func(float64, float64) { ended++ }
	s.Disable()

	s.Tapped(tapAt(s.Size().Width*0.5, mid))
	assert.Equal(t, 20.0, s.LowValue, "a tap must not move a disabled thumb")
	assert.Equal(t, 80.0, s.HighValue)

	s.Dragged(&event.DragEvent{PointEvent: *tapAt(s.Size().Width*0.5, mid)})
	s.DragEnd()
	assert.Equal(t, 20.0, s.LowValue, "a drag must not move a disabled thumb")
	assert.Equal(t, 80.0, s.HighValue)
	assert.False(t, s.Dragging(), "a disabled slider never enters a drag")

	assert.Zero(t, changed)
	assert.Zero(t, ended)
}

func TestRangeSlider_Disabled_IgnoresKeyboard(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.Step = 10
	s.SetValuesSilently(20, 80)
	s.Disable()

	changed := 0
	s.OnChanged = func(float64, float64) { changed++ }

	s.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	s.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	assert.Equal(t, 20.0, s.LowValue)
	assert.Equal(t, 80.0, s.HighValue)
	assert.Zero(t, changed)
}

// OnHover is suppressed while disabled, but OnHoverEnd still fires so a
// consumer disabled mid-hover is never left showing a preview it cannot take
// down -- the same rule Slider follows.
func TestRangeSlider_Disabled_HoverEndStillFires(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	sizedRangeSlider(t, s, 400, 60)
	mid := s.Size().Height / 2

	var hovers []float64
	s.OnHover = func(v float64) { hovers = append(hovers, v) }
	ends := 0
	s.OnHoverEnd = func() { ends++ }

	s.HoverIn(&event.HoverEvent{PointEvent: *tapAt(s.Size().Width*0.5, mid)})
	require.Len(t, hovers, 1)

	s.Disable()
	s.HoverMoved(&event.HoverEvent{PointEvent: *tapAt(s.Size().Width*0.75, mid)})
	assert.Len(t, hovers, 1, "a disabled slider reports no hover position")

	s.HoverOut()
	assert.Equal(t, 1, ends, "OnHoverEnd fires even when disabled mid-hover")
}

// --- Keyboard (#51) ---

func TestRangeSlider_TypedKey_MovesFocusedThumb(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.Step = 10
	s.SetValuesSilently(20, 80)

	assert.False(t, s.FocusedHigh(), "the low thumb is focused by default")

	s.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, 30.0, s.LowValue)
	assert.Equal(t, 80.0, s.HighValue, "the unfocused thumb stays put")

	s.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	assert.Equal(t, 20.0, s.LowValue)

	// The cross-axis arrows switch which thumb the arrows move.
	s.TypedKey(&event.KeyEvent{Name: event.KeyUp})
	assert.True(t, s.FocusedHigh())

	s.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, 20.0, s.LowValue)
	assert.Equal(t, 90.0, s.HighValue)

	s.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	assert.False(t, s.FocusedHigh())
}

func TestRangeSlider_TypedKey_Vertical(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.Orientation = widget.Vertical
	s.Step = 10
	s.SetValuesSilently(20, 80)

	// Up/Down adjust along the slider's own axis when vertical.
	s.TypedKey(&event.KeyEvent{Name: event.KeyUp})
	assert.Equal(t, 30.0, s.LowValue)
	s.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	assert.Equal(t, 20.0, s.LowValue)

	// Left/Right switch thumbs when vertical.
	s.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.True(t, s.FocusedHigh())
	s.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	assert.False(t, s.FocusedHigh())
}

// A keyboard adjustment is a complete interaction, so it commits.
func TestRangeSlider_TypedKey_FiresBothCallbacks(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.Step = 10
	s.SetValuesSilently(20, 80)

	changed, ended := 0, 0
	s.OnChanged = func(float64, float64) { changed++ }
	s.OnChangeEnded = func(float64, float64) { ended++ }

	s.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, 1, changed)
	assert.Equal(t, 1, ended)

	// A key that cannot move the thumb any further reports nothing.
	s.SetValuesSilently(0, 100)
	changed, ended = 0, 0
	s.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	assert.Zero(t, changed)
	assert.Zero(t, ended)
}

// Without an explicit Step the arrows still work, stepping a hundredth of the
// range -- the same fallback the screen-reader increment uses.
func TestRangeSlider_TypedKey_NoStep(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.SetValuesSilently(20, 80)

	s.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, 21.0, s.LowValue)
}

// The focused thumb follows the pointer, so arrows continue where a drag or tap
// left off.
func TestRangeSlider_FocusedThumbFollowsPointer(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.Step = 10
	s.SetValuesSilently(20, 80)
	sizedRangeSlider(t, s, 400, 60)
	mid := s.Size().Height / 2

	s.Tapped(tapAt(s.Size().Width*0.9, mid))
	assert.True(t, s.FocusedHigh(), "tapping near the high thumb focuses it")

	s.Tapped(tapAt(s.Size().Width*0.05, mid))
	assert.False(t, s.FocusedHigh(), "tapping near the low thumb focuses it")

	s.Dragged(&event.DragEvent{PointEvent: *tapAt(s.Size().Width*0.95, mid)})
	s.DragEnd()
	assert.True(t, s.FocusedHigh(), "dragging the high thumb focuses it")
}

func TestRangeSlider_SetFocusedHigh(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	s.Step = 10
	s.SetValuesSilently(20, 80)

	s.SetFocusedHigh(true)
	assert.True(t, s.FocusedHigh())

	s.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, 90.0, s.HighValue)
	assert.Equal(t, 20.0, s.LowValue)
}

func TestRangeSlider_Focusable(t *testing.T) {
	test.NewTempApp(t)
	s := widget.NewRangeSlider(0, 100)
	sizedRangeSlider(t, s, 400, 60)

	s.FocusGained()
	s.FocusLost()
	s.TypedRune('a') // must not panic or move anything
	assert.Equal(t, 0.0, s.LowValue)
	assert.Equal(t, 100.0, s.HighValue)
}
