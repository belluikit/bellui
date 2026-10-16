//go:build !windows || !ci

package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/widget"
)

// The canvas file dialog's touch selection mode (#259) rides this dispatch
// geometry: its file items sit inside a scroll container inside a modal
// pop-up, and the hold-fire long-press (pre-rebrand tracker #758) must resolve the innermost
// LongPressable there — not the scroll, not the pop-up. The release of a
// hold-fired touch must also stay tap-free: in a toggle-on-tap mode a release
// tap would immediately undo the selection the hold just made.
func Test_canvas_LongPressResolvesThroughModalScroll(t *testing.T) {
	r := newLongPressRecorder()
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(widget.NewLabel("content behind"))
	c.Resize(geo.NewSize(200, 200))

	pop := widget.NewModalPopUp(container.NewScroll(r), c)
	pop.Resize(geo.NewSize(120, 120))
	pop.Show()
	c.EnsureMinSize()

	var pos geo.Position
	found := false
	intdriver.WalkVisibleObjectTree(pop, func(obj core.CanvasObject, p geo.Position, _ geo.Position, _ geo.Size) bool {
		if obj == core.CanvasObject(r) {
			pos, found = p, true
		}
		return false
	}, nil)
	if !found {
		t.Fatal("recorder not found in the popup tree")
	}

	tap := pos.AddXY(4, 4)
	c.TapDown(tap, 0)
	waitAndCheck(400, func() {
		assert.Equal(t, 1, r.longPressed, "hold-fire resolves the item through the pop-up and scroll")
	})

	lprTapUp(c, tap)
	waitAndCheck(100, func() {
		assert.Equal(t, 1, r.longPressed, "release does not re-fire the long-press")
		assert.Zero(t, r.tapped, "the release of a hold-fired touch must not deliver Tapped")
	})
}
