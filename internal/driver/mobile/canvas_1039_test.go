package mobile

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

// Regression test for pre-rebrand tracker #1039: Border side objects must keep their laid-out
// width after the Border is detached from the window tree and re-attached
// (the goed App Map toggle shape).
func Test_canvas_BorderSideAfterReattach(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)

	toolbar := widget.NewLabel("toolbar")
	status := widget.NewLabel("status")
	center := belluicanvas.NewRectangle(color.Black)
	props := widget.NewLabel("props")
	side := container.NewVScroll(props)
	side.SetMinSize(geo.NewSize(240, 0))

	border := container.NewBorder(toolbar, status, nil, side, center)
	editor := widget.NewLabel("editor")

	split := container.NewHSplit(border, widget.NewLabel("trailing"))
	split.Offset = 0.8

	c.SetContent(split)
	c.Resize(geo.NewSize(800, 600))
	c.EnsureMinSize()

	assert.GreaterOrEqual(t, side.Size().Width, float32(240), "first attachment")

	// Toggle off: swap the editor into the split slot.
	split.Leading = editor
	split.Refresh()
	c.EnsureMinSize()

	// Toggle on: re-attach the same Border instance.
	split.Leading = border
	split.Refresh()
	c.EnsureMinSize()

	assert.GreaterOrEqual(t, side.Size().Width, float32(240), "after re-attachment")
}

// pre-rebrand tracker #1039: a min-size change must re-lay-out its parent even when another
// min-size change happens in a later sibling's subtree in the same frame.
// The old EnsureMinSize kept a single pending-parent pointer, so the later
// (deeper) change overwrote the earlier sibling's pending parent update; if
// the deeper change was absorbed by an intermediate widget whose own min
// size stayed put (a Scroll here), the first parent never re-laid and the
// stale child size stuck permanently.
func Test_canvas_MinSizeChangesInSiblingSubtreesBothApply(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)

	first := belluicanvas.NewRectangle(color.Black)
	first.SetMinSize(geo.NewSize(100, 50))

	inner := belluicanvas.NewRectangle(color.Black)
	inner.SetMinSize(geo.NewSize(50, 50))
	// A both-axes Scroll reports a constant min size, so a change to
	// inner's min size never propagates above the scroll.
	absorber := container.NewScroll(inner)

	// The Stack wrapper always hands the HBox the full canvas size, so the
	// HBox's own size never changes — recovering first's width can only
	// happen through the HBox's own pending re-layout.
	content := container.NewHBox(first, absorber)
	c.SetContent(container.NewStack(content))
	c.Resize(geo.NewSize(400, 300))
	c.EnsureMinSize()

	assert.Equal(t, float32(100), first.Size().Width)

	// Both change in the same frame: first (earlier sibling) grows, and so
	// does a node buried under the later sibling's min-size-absorbing scroll.
	first.SetMinSize(geo.NewSize(120, 60))
	inner.SetMinSize(geo.NewSize(60, 60))
	c.EnsureMinSize()

	assert.Equal(t, float32(120), first.Size().Width,
		"HBox must re-lay-out for the earlier sibling's min size change")
}
