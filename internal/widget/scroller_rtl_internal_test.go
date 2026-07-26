package widget

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

func rtlScroll(t *testing.T, dir bellui.Direction) {
	t.Helper()
	bellui.SetLayoutDirection(dir)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })
}

func newWideScroll() *Scroll {
	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(1000, 50))
	scroll := NewHScroll(rect)
	scroll.Resize(geo.NewSize(100, 100))
	return scroll
}

func TestScrollContainer_RTL_StartsAtRightEdge(t *testing.T) {
	rtlScroll(t, bellui.DirectionRTL)

	scroll := newWideScroll()
	// 1000 (content) - 100 (viewport) = 900: the content's right edge, which is
	// the reading start under RTL.
	assert.Equal(t, float32(900), scroll.Offset.X)
}

func TestScrollContainer_LTR_StartsAtLeftEdge(t *testing.T) {
	rtlScroll(t, bellui.DirectionLTR)

	scroll := newWideScroll()
	assert.Equal(t, float32(0), scroll.Offset.X, "LTR scrolling is unchanged")
}

func TestScrollContainer_RTL_FlipsWheel(t *testing.T) {
	rtlScroll(t, bellui.DirectionRTL)

	scroll := newWideScroll()
	require.Equal(t, float32(900), scroll.Offset.X)

	// The same wheel delta that advances rightward (+10) under LTR advances
	// leftward under RTL, moving toward the content's far (left) end.
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(-10, 0)})
	assert.Equal(t, float32(890), scroll.Offset.X)
}

func TestScrollContainer_RTL_FlipsVerticalWheelMappedToHorizontal(t *testing.T) {
	rtlScroll(t, bellui.DirectionRTL)

	scroll := newWideScroll()
	require.Equal(t, float32(900), scroll.Offset.X)

	// A vertical wheel over narrow horizontal content is redirected to the
	// horizontal axis, then reversed for RTL.
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -10)})
	assert.Equal(t, float32(890), scroll.Offset.X)
}

func TestScrollContainer_LTR_WheelUnchanged(t *testing.T) {
	rtlScroll(t, bellui.DirectionLTR)

	scroll := newWideScroll()
	require.Equal(t, float32(0), scroll.Offset.X)

	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(-10, 0)})
	assert.Equal(t, float32(10), scroll.Offset.X)
}

func TestScrollContainer_RTL_UserScrollSupersedesPin(t *testing.T) {
	rtlScroll(t, bellui.DirectionRTL)

	scroll := newWideScroll()
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(-10, 0)}) // user scrolls to 890
	require.Equal(t, float32(890), scroll.Offset.X)

	// A relayout must preserve the user's position, not re-pin to the right edge
	// (which after this resize would be 1000-80 = 920).
	scroll.Resize(geo.NewSize(80, 100))
	assert.Equal(t, float32(890), scroll.Offset.X)
}
