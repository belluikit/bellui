//go:build mobile

package container

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// #263: the resize corner spans the touch-target floor on mobile while the
// visual indicator keeps its desktop size, and the layout keeps that target
// clear of the title-bar buttons when a window is shrunk to minimal content.
func TestInnerWindow_CornerTouchTargetMobile(t *testing.T) {
	test.NewTempApp(t)

	w := NewInnerWindow("Title", widget.NewLabel("x"))
	th := w.Theme()
	target := th.Size(theme.SizeNameIconButtonSize) + th.Size(theme.SizeNamePadding)*2

	corner := newDraggableCorner(w)
	assert.Equal(t, geo.NewSquareSize(target), corner.MinSize(),
		"the corner drag target must meet the touch floor on mobile")

	// Roomy window: the corner spans the full target.
	r := test.TempWidgetRenderer(t, w).(*innerWindowRenderer)
	w.Resize(geo.NewSize(300, 300))
	assert.Equal(t, geo.NewSquareSize(target), r.corner.Size())

	// Window shrunk to minimal content: the target is capped below the title
	// bar rather than stacked over the frame buttons.
	w.Resize(w.MinSize())
	assert.GreaterOrEqual(t, r.corner.Position().Y, w.barHeight(),
		"the corner must stay clear of the title bar at minimum size")
}
