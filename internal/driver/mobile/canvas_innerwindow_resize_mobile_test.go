//go:build mobile && (!windows || !ci) && !wasm

package mobile

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

func resizeDrag(wid event.Draggable, ev *event.DragEvent) {
	wid.Dragged(ev)
}

// showResizableDialogFrame replicates dialog.wrapModal's windowed structure
// with the resize wiring live (wrapModal sets OnResized unconditionally, so
// the corner is active in every windowed dialog) and scrollable content
// reaching the bottom-right — the file dialog's worst-case layout.
func showResizableDialogFrame(c *canvas) (*widget.PopUp, *container.Scroll) {
	scroll := container.NewScroll(widget.NewLabel(strings.Repeat("line\n", 100)))
	win := container.NewInnerWindow("", scroll)
	win.Alignment = widget.ButtonAlignTrailing
	pop := widget.NewModalPopUp(win, c)
	pop.SetPadded(false)
	win.OnMaximized = func() {}
	win.OnResized = func(ev *event.DragEvent) {
		pop.Resize(win.Size().Add(geo.NewSize(ev.Dragged.DX, ev.Dragged.DY)).Max(win.MinSize()))
	}
	pop.Resize(geo.NewSize(300, 300))
	pop.Show()
	c.EnsureMinSize()
	return pop, scroll
}

// innerWindowRect locates the InnerWindow's absolute position and size in the
// popup tree.
func innerWindowRect(pop *widget.PopUp) (geo.Position, geo.Size, bool) {
	var winPos geo.Position
	var winSize geo.Size
	found := false
	intdriver.WalkVisibleObjectTree(pop, func(obj core.CanvasObject, pos geo.Position, _ geo.Position, _ geo.Size) bool {
		if _, ok := obj.(*container.InnerWindow); ok {
			winPos, winSize, found = pos, obj.Size(), true
		}
		return false
	}, nil)
	return winPos, winSize, found
}

// Test_windowedDialogResizeCorner_TouchTarget covers #263: the InnerWindow
// resize corner was a 16dp drag target — under every platform's touch floor,
// and the drivers additionally shift each touch up by tapYOffset before
// hit-testing, the same geometry that made the #254 close button dead on a
// device. On mobile the corner must span the frame-button touch target, a
// drag starting in it must resize the window, and — because the drag latch
// resolves at the gesture origin — a scroll starting outside the corner must
// keep scrolling the content beneath.
func Test_windowedDialogResizeCorner_TouchTarget(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(widget.NewLabel("content behind"))
	c.Resize(geo.NewSize(600, 600))

	pop, scroll := showResizableDialogFrame(c)
	winPos, winSize, found := innerWindowRect(pop)
	if !found {
		t.Fatal("no InnerWindow found in the popup tree")
	}

	th := theme.Current()
	target := th.Size(theme.SizeNameIconButtonSize) + th.Size(theme.SizeNamePadding)*2
	bottomRight := winPos.Add(geo.NewPos(winSize.Components()))

	// The drag target resolved at the corner must span the platform floor.
	probe := bottomRight.AddXY(-6, -6)
	corner, _, _ := c.FindObjectAtPositionMatching(probe, func(object core.CanvasObject) bool {
		_, ok := object.(event.Draggable)
		return ok
	})
	if assert.NotNil(t, corner, "a draggable must resolve at the window corner") {
		assert.GreaterOrEqual(t, corner.Size().Width, target, "corner drag-target width must meet the touch floor")
		assert.GreaterOrEqual(t, corner.Size().Height, target, "corner drag-target height must meet the touch floor")
	}

	// A finger grabbing the visual indicator: the driver shifts the touch up
	// by 8dp (tapYOffset) before hit-testing, so start where such a touch
	// actually resolves and drag outward. The window must grow. (pop.Size()
	// is the full-canvas overlay; the InnerWindow is what resizes.)
	before := winSize
	start := bottomRight.AddXY(-8, -16)
	c.TapDown(start, 0)
	c.TapMove(start.AddXY(16, 16), 0, resizeDrag)
	c.TapMove(start.AddXY(40, 40), 0, resizeDrag)
	lprTapUp(c, start.AddXY(40, 40))
	_, after, _ := innerWindowRect(pop)
	assert.Greater(t, after.Width, before.Width, "a drag from the corner must grow the window")
	assert.Greater(t, after.Height, before.Height, "a drag from the corner must grow the window")
	assert.Equal(t, geo.NewPos(0, 0), scroll.Offset, "a corner drag must not scroll the content")

	// The deliberate limit of the enlarged target: only drags *starting*
	// inside the corner square resize. A scroll starting above it — even
	// hugging the same edge — must still latch the content scroll.
	winPos, winSize, _ = innerWindowRect(pop)
	bottomRight = winPos.Add(geo.NewPos(winSize.Components()))
	scrollStart := geo.NewPos(bottomRight.X-16, bottomRight.Y-target-12)
	c.TapDown(scrollStart, 1)
	c.TapMove(scrollStart.AddXY(0, -20), 1, resizeDrag)
	c.TapMove(scrollStart.AddXY(0, -60), 1, resizeDrag)
	assert.Positive(t, scroll.Offset.Y, "a drag starting outside the corner target must scroll the content")
	_, final, _ := innerWindowRect(pop)
	assert.Equal(t, winSize, final, "a content scroll must not resize the window")

	// Release the second gesture too, so the handler's drag latch does not
	// outlive the test (lprTapUp is pinned to pointer 0; this gesture is 1).
	c.TapUp(scrollStart.AddXY(0, -60), 1, func(event.Tappable, *event.PointEvent) {
	}, func(event.SecondaryTappable, *event.PointEvent) {
	}, func(event.LongPressable, *event.PointEvent) {
	}, func(event.DoubleTappable, *event.PointEvent) {
	}, func(wid event.Draggable, _ *event.DragEvent) {
		wid.DragEnd()
	})
}
