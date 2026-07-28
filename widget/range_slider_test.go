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
