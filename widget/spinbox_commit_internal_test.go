package widget

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
	"github.com/stretchr/testify/assert"
)

// spinBoxIn shows a spin box in a test window and counts its OnChanged calls.
func spinBoxIn(t *testing.T, min, max, step float64) (*SpinBox, bellui.Window, *int) {
	t.Helper()
	test.NewTempApp(t)
	s := NewSpinBox(min, max, step)
	w := test.NewTempWindow(t, s)
	calls := 0
	s.OnChanged = func(float64) { calls++ }
	return s, w, &calls
}

// typeInto focuses the field through the canvas, as a tap does, and types
// text into it. Focus selects the field's text, so the first rune replaces
// it. test.Type is not used: it focuses the entry behind the canvas' back,
// so the canvas never unfocuses it, and it re-focuses on every call.
func typeInto(w bellui.Window, s *SpinBox, text string) {
	if w.Canvas().Focused() != s.entry {
		w.Canvas().Focus(s.entry)
	}
	for _, r := range text {
		s.entry.TypedRune(r)
	}
}

// Typed text out of range is corrected when the user is done with it, on
// focus loss or Enter, whichever way the field was focused (#1440).
func TestSpinBox_CommitOutOfRangeText(t *testing.T) {
	for _, tc := range []struct {
		name string
		done func(s *SpinBox, w bellui.Window)
	}{
		{"focus loss", func(_ *SpinBox, w bellui.Window) { w.Canvas().Unfocus() }},
		{"enter", func(s *SpinBox, _ bellui.Window) { s.entry.TypedKey(&event.KeyEvent{Name: event.KeyReturn}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, w, calls := spinBoxIn(t, 0, 10, 1)
			typeInto(w, s, "12")
			assert.Equal(t, 10.0, s.Value, "typing already clamps Value")
			assert.Equal(t, "12", s.entry.Text, "not corrected while typing")
			before := *calls
			tc.done(s, w)
			assert.Equal(t, "10", s.entry.Text)
			assert.Equal(t, 10.0, s.Value)
			assert.Equal(t, before, *calls, "the correction does not report: Value did not change")
		})
	}
}

// Focused by Tab, the spin box itself holds focus and passes keys to its
// field; its focus loss corrects the text too (#1440).
func TestSpinBox_CommitWhenTheBoxHoldsFocus(t *testing.T) {
	s, w, _ := spinBoxIn(t, 0, 10, 1)
	w.Canvas().Focus(s)
	s.entry.SetText("")
	s.TypedRune('1')
	s.TypedRune('2')
	assert.Equal(t, "12", s.entry.Text)
	w.Canvas().Unfocus()
	assert.Equal(t, "10", s.entry.Text)
}

// Correcting on every keystroke would fight typing: in 10–20 the "1" of
// "15" is below Min. The text stays until the user is done.
func TestSpinBox_NoCorrectionMidTyping(t *testing.T) {
	s, w, _ := spinBoxIn(t, 10, 20, 1)
	typeInto(w, s, "1")
	assert.Equal(t, "1", s.entry.Text)
	typeInto(w, s, "5")
	assert.Equal(t, "15", s.entry.Text)
	w.Canvas().Unfocus()
	assert.Equal(t, "15", s.entry.Text)
	assert.Equal(t, 15.0, s.Value)
}

// Loose or empty text is shown as Value formatted for Step; text that is
// exactly Value stays when Step's format would round it away.
func TestSpinBox_CommitNormalizes(t *testing.T) {
	t.Run("loose", func(t *testing.T) {
		s, w, _ := spinBoxIn(t, 0, 10, 1)
		typeInto(w, s, "3.")
		w.Canvas().Unfocus()
		assert.Equal(t, "3", s.entry.Text)
	})
	t.Run("empty", func(t *testing.T) {
		s, w, _ := spinBoxIn(t, 0, 10, 1)
		s.SetValue(4)
		w.Canvas().Focus(s.entry)
		s.entry.SetText("")
		w.Canvas().Unfocus()
		assert.Equal(t, "4", s.entry.Text)
		assert.Equal(t, 4.0, s.Value)
	})
	t.Run("kept when the format would round it", func(t *testing.T) {
		s, w, _ := spinBoxIn(t, 0, 10, 1)
		typeInto(w, s, "2.5")
		w.Canvas().Unfocus()
		assert.Equal(t, 2.5, s.Value)
		assert.Equal(t, "2.5", s.entry.Text, "not shown as 2 while Value is 2.5")
	})
}

// A screen reader reads what the field shows (#1440).
func TestSpinBox_AccessibleValueMatchesField(t *testing.T) {
	for _, tc := range []struct {
		min, max, step, value float64
		want                  string
	}{
		{0, 5, 0.25, 2.25, "2.25"},
		{0, 10, 1.5, 1.5, "1.5"},
		{0, 10, 1, 7, "7"},
	} {
		s, _, _ := spinBoxIn(t, tc.min, tc.max, tc.step)
		s.SetValue(tc.value)
		assert.Equal(t, s.entry.Text, s.AccessibleValue(), "step %v", tc.step)
		assert.Equal(t, tc.want, s.AccessibleValue(), "step %v", tc.step)
	}
	t.Run("reads the field where the format would round", func(t *testing.T) {
		s, w, _ := spinBoxIn(t, 0, 10, 1)
		typeInto(w, s, "2.5")
		w.Canvas().Unfocus()
		assert.Equal(t, "2.5", s.AccessibleValue())
	})
	t.Run("while typing out of range it reads Value", func(t *testing.T) {
		s, w, _ := spinBoxIn(t, 0, 10, 1)
		typeInto(w, s, "12")
		assert.Equal(t, "10", s.AccessibleValue())
	})
}
