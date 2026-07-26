package widget_test

import (
	"math"
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
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
