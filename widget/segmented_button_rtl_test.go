package widget_test

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestSegmentedButton_ArrowKeys_LTR(t *testing.T) {
	test.NewTempApp(t)
	bellui.SetLayoutDirection(bellui.DirectionLTR)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	changed := -1
	sb := widget.NewSegmentedButton([]string{"A", "B", "C"}, func(i int) { changed = i })
	sb.Selected = 1

	sb.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, 2, sb.Selected, "LTR KeyRight advances")
	assert.Equal(t, 2, changed)

	sb.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	assert.Equal(t, 1, sb.Selected, "LTR KeyLeft retreats")

	// Clamps at the ends (no wrap).
	sb.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	sb.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, 2, sb.Selected)
}

func TestSegmentedButton_ArrowKeys_RTL(t *testing.T) {
	test.NewTempApp(t)
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	sb := widget.NewSegmentedButton([]string{"A", "B", "C"}, nil)
	sb.Selected = 1

	// Under RTL the visual order is reversed, so the right arrow steps to the
	// previous segment and the left arrow to the next.
	sb.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, 0, sb.Selected, "RTL KeyRight retreats")

	sb.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	assert.Equal(t, 1, sb.Selected, "RTL KeyLeft advances")
	sb.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	assert.Equal(t, 2, sb.Selected)
}

func TestSegmentedButton_FocusableNoPanic(t *testing.T) {
	test.NewTempApp(t)
	sb := widget.NewSegmentedButton([]string{"A", "B"}, nil)
	assert.NotPanics(t, func() {
		sb.FocusGained()
		sb.TypedRune('x')
		sb.FocusLost()
	})
}
