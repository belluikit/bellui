package widget_test

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestSelect_ArrowKeys_RTL(t *testing.T) {
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	sel := widget.NewSelect([]string{"A", "B", "C"}, nil)
	sel.SetSelectedIndex(1) // "B"

	// Under RTL the right arrow steps to the previous option and the left arrow
	// to the next, mirroring the visual reading order.
	sel.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, "A", sel.Selected)

	sel.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	assert.Equal(t, "B", sel.Selected)
	sel.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	assert.Equal(t, "C", sel.Selected)
}
