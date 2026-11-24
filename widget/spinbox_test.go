package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

func TestSpinBox_Creation(t *testing.T) {
	s := NewSpinBox(0, 100, 1)
	assert.NotNil(t, s)
	assert.Equal(t, 0.0, s.Value)
	assert.Equal(t, 0.0, s.Min)
	assert.Equal(t, 100.0, s.Max)
	assert.Equal(t, 1.0, s.Step)
}

func TestSpinBox_SetValue(t *testing.T) {
	s := NewSpinBox(0, 100, 1)
	w := test.NewTempWindow(t, s)
	_ = w

	s.SetValue(50)
	assert.Equal(t, 50.0, s.Value)
	assert.Equal(t, "50", s.entry.Text)
}

func TestSpinBox_SetValueClamps(t *testing.T) {
	s := NewSpinBox(0, 100, 1)
	w := test.NewTempWindow(t, s)
	_ = w

	s.SetValue(200)
	assert.Equal(t, 100.0, s.Value)

	s.SetValue(-10)
	assert.Equal(t, 0.0, s.Value)
}

func TestSpinBox_Increment(t *testing.T) {
	var changed float64
	s := NewSpinBox(0, 10, 1)
	s.OnChanged = func(val float64) { changed = val }
	w := test.NewTempWindow(t, s)
	_ = w

	test.Tap(s.incBtn)
	assert.Equal(t, 1.0, s.Value)
	assert.Equal(t, 1.0, changed)
}

func TestSpinBox_Decrement(t *testing.T) {
	s := NewSpinBox(0, 10, 1)
	w := test.NewTempWindow(t, s)
	_ = w

	s.SetValue(5)
	test.Tap(s.decBtn)
	assert.Equal(t, 4.0, s.Value)
}

func TestSpinBox_IncrementAtMax(t *testing.T) {
	s := NewSpinBox(0, 10, 1)
	w := test.NewTempWindow(t, s)
	_ = w

	s.SetValue(10)
	assert.True(t, s.incBtn.Disabled(), "Increment should be disabled at max")

	test.Tap(s.incBtn)
	assert.Equal(t, 10.0, s.Value, "Value should not exceed max")
}

func TestSpinBox_DecrementAtMin(t *testing.T) {
	s := NewSpinBox(0, 10, 1)
	w := test.NewTempWindow(t, s)
	_ = w

	assert.True(t, s.decBtn.Disabled(), "Decrement should be disabled at min")

	test.Tap(s.decBtn)
	assert.Equal(t, 0.0, s.Value, "Value should not go below min")
}

func TestSpinBox_FloatStep(t *testing.T) {
	s := NewSpinBox(0, 1, 0.1)
	w := test.NewTempWindow(t, s)
	_ = w

	s.SetValue(0.5)
	assert.Equal(t, "0.5", s.entry.Text)
}

func TestSpinBox_KeyboardIncrement(t *testing.T) {
	s := NewSpinBox(0, 10, 1)
	w := test.NewTempWindow(t, s)
	_ = w

	s.SetValue(5)
	s.entry.TypedKey(&event.KeyEvent{Name: event.KeyUp})
	assert.Equal(t, 6.0, s.Value)

	s.entry.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	assert.Equal(t, 5.0, s.Value)
}

func TestSpinBox_OnChanged(t *testing.T) {
	calls := 0
	s := NewSpinBox(0, 100, 1)
	s.OnChanged = func(val float64) { calls++ }
	w := test.NewTempWindow(t, s)
	_ = w

	s.SetValue(10)
	assert.Equal(t, 1, calls)

	s.SetValue(10) // same value, no callback
	assert.Equal(t, 1, calls)

	s.SetValue(20)
	assert.Equal(t, 2, calls)
}

func TestSpinBox_Disable(t *testing.T) {
	s := NewSpinBox(0, 10, 1)
	w := test.NewTempWindow(t, s)
	_ = w

	s.SetValue(5)
	s.Disable()
	assert.True(t, s.Disabled())
	assert.True(t, s.entry.Disabled())
	assert.True(t, s.incBtn.Disabled())
	assert.True(t, s.decBtn.Disabled())

	s.Enable()
	assert.False(t, s.Disabled())
	assert.False(t, s.entry.Disabled())
}

func TestSpinBox_MinSize(t *testing.T) {
	s := NewSpinBox(0, 100, 1)
	w := test.NewTempWindow(t, s)
	_ = w

	min := s.MinSize()
	assert.Greater(t, min.Width, float32(0))
	assert.Greater(t, min.Height, float32(0))
}

func TestSpinBox_IntegerFormat(t *testing.T) {
	s := NewSpinBox(0, 100, 5)
	w := test.NewTempWindow(t, s)
	_ = w

	s.SetValue(25)
	assert.Equal(t, "25", s.entry.Text)
}

func TestSpinBox_ParseAndUpdate_ClampsTypedValue(t *testing.T) {
	calls := 0
	s := NewSpinBox(0, 100, 1)
	s.OnChanged = func(float64) { calls++ }
	test.NewTempWindow(t, s)

	// Typing a value beyond Max (drives the entry's OnChanged → parseAndUpdate)
	// clamps the value and fires OnChanged once.
	s.entry.SetText("150")
	assert.Equal(t, 100.0, s.Value)
	assert.Equal(t, 1, calls)
}

func TestSpinBox_ParseAndUpdate_IgnoresInvalid(t *testing.T) {
	calls := 0
	s := NewSpinBox(0, 100, 1)
	s.OnChanged = func(float64) { calls++ }
	test.NewTempWindow(t, s)

	s.parseAndUpdate("not-a-number")
	assert.Equal(t, 0.0, s.Value)
	assert.Equal(t, 0, calls)
}

func TestSpinBox_SetValue_DoesNotDoubleFire(t *testing.T) {
	calls := 0
	s := NewSpinBox(0, 100, 1)
	s.OnChanged = func(float64) { calls++ }
	test.NewTempWindow(t, s)

	// SetValue → entry.SetText → entry.OnChanged → parseAndUpdate is suppressed
	// by the settingValue guard, so OnChanged fires exactly once, not twice.
	s.SetValue(42)
	assert.Equal(t, 1, calls)
}

func TestSpinBox_FormatValue_FractionalStep(t *testing.T) {
	for _, tt := range []struct {
		step, value float64
		want        string
	}{
		{0.25, 0.5, "0.50"},
		{0.05, 0.1, "0.10"},
	} {
		s := NewSpinBox(0, 1, tt.step)
		test.NewTempWindow(t, s)
		s.SetValue(tt.value)
		assert.Equal(t, tt.want, s.entry.Text, "step %v value %v", tt.step, tt.value)
	}
}

// assertSpinBoxFieldFits sizes the box to its minimum while it shows a short
// value, then steps it to each of values without resizing and checks the
// number field holds the whole text without scrolling: the text and its
// padding are no wider than the field's viewport. The box must be sized from
// its range, not from the value it shows now (#1420).
func assertSpinBoxFieldFits(t *testing.T, s *SpinBox, values ...float64) {
	t.Helper()
	w := test.NewTempWindow(t, s)
	size := s.MinSize()
	w.Resize(size.AddWidthHeight(theme.Padding()*2, theme.Padding()*2))
	s.Resize(size)
	for _, v := range values {
		s.SetValue(v)
		assert.Equal(t, size, s.MinSize(), "the box keeps its size as the value changes")

		border := s.entry.Theme().Size(theme.SizeNameInputBorder)
		viewport := s.entry.Size().Width - border*2
		need := s.entry.textProvider().MinSize().Width
		assert.LessOrEqualf(t, need, viewport,
			"%q needs %.1f but the field shows %.1f (box min width %.1f)", s.entry.Text, need, viewport, size.Width)
	}
}

func TestSpinBox_FieldFitsWideRange(t *testing.T) {
	test.NewTempApp(t).Settings().SetTheme(theme.DefaultTheme())
	s := NewSpinBox(-100000, 100000, 1)
	s.SetValue(0)
	assertSpinBoxFieldFits(t, s, -100000, 100000, -99999)
}

func TestSpinBox_FieldFitsDecimalStep(t *testing.T) {
	test.NewTempApp(t).Settings().SetTheme(theme.DefaultTheme())
	s := NewSpinBox(0, 1000, 0.25)
	assertSpinBoxFieldFits(t, s, 999.75, 1000)
}

func TestSpinBox_FieldWidensWhenRangeChanges(t *testing.T) {
	test.NewTempApp(t).Settings().SetTheme(theme.DefaultTheme())
	s := NewSpinBox(0, 10, 1)
	test.NewTempWindow(t, s)
	narrow := s.MinSize().Width

	s.Max = 10000000
	s.Refresh()
	assert.Greater(t, s.MinSize().Width, narrow, "a wider Max widens the box after Refresh")
	assertSpinBoxFieldFits(t, s, 10000000)
}
