package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// TestSegmentedButton_TapMovesFocus verifies that tapping a segment moves
// keyboard focus to the parent SegmentedButton so that click-then-arrow
// navigation works without first Tabbing to the group (pre-rebrand tracker #174).
func TestSegmentedButton_TapMovesFocus(t *testing.T) {
	test.NewTempApp(t)

	sb := NewSegmentedButton([]string{"A", "B", "C"}, nil)
	w := test.NewTempWindow(t, sb)
	w.Resize(geo.NewSize(300, 80))
	c := w.Canvas()

	r := test.TempWidgetRenderer(t, sb).(*segButtonRenderer)

	// Nothing focused to start.
	assert.Nil(t, c.Focused())

	// Tap the second segment: it always selects.
	r.segments[1].Tapped(&event.PointEvent{})
	assert.Equal(t, 1, sb.Selected)

	if bellui.CurrentDevice().IsMobile() {
		// Focus is intentionally not taken on mobile (focusIfNotMobile).
		assert.Nil(t, c.Focused())
		return
	}

	// On desktop the group takes focus, so an arrow key cycles the selection
	// without a prior Tab.
	assert.Equal(t, sb, c.Focused())
	sb.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, 2, sb.Selected)
}
