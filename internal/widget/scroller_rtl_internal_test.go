package widget

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/theme"
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

func TestScrollContainer_RTL_TouchScrollDirectionAbsolute(t *testing.T) {
	rtlScroll(t, bellui.DirectionRTL)

	scroll := newWideScroll()
	require.Equal(t, float32(900), scroll.Offset.X)

	// A wheel scroll is RTL-resolved: positive DX flips, staying clamped at
	// the right-edge reading start.
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(30, 0), Source: event.ScrollSourceMouseWheel})
	assert.Equal(t, float32(900), scroll.Offset.X)

	// A direct-touch scroll (#291) is direction-absolute: the content tracks
	// the finger — a rightwards drag reveals content to the left, RTL or not —
	// exactly as the mobile driver's drag path behaves.
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(30, 0), Source: event.ScrollSourceTouch})
	assert.Equal(t, float32(870), scroll.Offset.X)
}

// A ScrollToBottom issued before the first layout — a log view appending rows
// while the window is still being built — clamps to nothing and must not cancel
// the RTL right-edge start the first layout applies. It did once ScrollToBottom
// went through ScrollToOffset, which pinned before it knew whether anything
// moved (review of #688).
func TestScrollContainer_RTL_EarlyScrollToBottomKeepsPin(t *testing.T) {
	rtlScroll(t, bellui.DirectionRTL)

	rect := canvas.NewRectangle(color.Black)
	rect.SetMinSize(geo.NewSize(1000, 1000))
	scroll := NewScroll(rect)
	scroll.ScrollToBottom()
	scroll.Resize(geo.NewSize(100, 100))
	assert.Equal(t, float32(900), scroll.Offset.X, "the RTL start pin was lost")
}

// scrollParts is where a Scroll draws its content, its vertical bar's area,
// the bar in it, and the bar's thin idle visual: each one's left edge and
// width in the Scroll's coordinates. Hidden parts are left out.
func scrollParts(s *Scroll) map[string][2]float32 {
	r := cache.Renderer(s).(*scrollContainerRenderer)
	got := map[string][2]float32{
		"content": {s.Content.Position().X, s.Content.Size().Width},
	}
	if !r.vertArea.Visible() {
		return got
	}
	area := r.vertArea
	bar := area.bar
	thin := cache.Renderer(bar).(*scrollBarRenderer).background
	got["bar area"] = [2]float32{area.Position().X, area.Size().Width}
	got["bar"] = [2]float32{area.Position().X + bar.Position().X, bar.Size().Width}
	got["bar visual"] = [2]float32{area.Position().X + bar.Position().X + thin.Position().X, thin.Size().Width}
	return got
}

// Right-to-left, a Scroll's vertical bar and the strip it reserves are at its
// left edge, and the content starts after the strip, so it still ends at the
// Scroll's right edge: every part is drawn where its left-to-right
// counterpart is, mirrored. That holds with a full strip, a partial one (the
// content needs more than the Scroll less the bar), no strip
// (ReserveBarSpace off), after a vertical scroll, once the content stops
// overflowing, and for content that overflows both ways. A touch device reserves no strip and overlays the
// bar, on the left right-to-left (#1496). The bar and its strip used to stay
// at the right.
func TestScrollContainer_RTL_VerticalBarAtLeft(t *testing.T) {
	const size = float32(100)
	barWidth := theme.Current().Size(theme.SizeNameScrollBar)
	require.Greater(t, barWidth, float32(1))
	mobile := bellui.CurrentDevice().IsMobile()

	cases := []struct {
		name    string
		content geo.Size
		make    func(core.CanvasObject) *Scroll
		reserve bool
		strip   float32 // the strip reserved on desktop
	}{
		{"strip", geo.NewSize(50, 1000), NewVScroll, true, barWidth},
		{"partial strip", geo.NewSize(size-barWidth/2, 1000), NewVScroll, true, barWidth / 2},
		{"no reserve", geo.NewSize(50, 1000), NewVScroll, false, 0},
		{"both ways", geo.NewSize(1000, 1000), NewScroll, true, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			layouts := map[bellui.Direction][]map[string][2]float32{}
			for _, dir := range []bellui.Direction{bellui.DirectionLTR, bellui.DirectionRTL} {
				bellui.SetLayoutDirection(dir)
				rect := canvas.NewRectangle(color.Black)
				rect.SetMinSize(c.content)
				s := c.make(rect)
				s.ReserveBarSpace = c.reserve
				s.Resize(geo.NewSize(size, size))
				layouts[dir] = append(layouts[dir], scrollParts(s))
				s.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -30)})
				require.Equal(t, float32(30), s.Offset.Y, "the content scrolled down")
				layouts[dir] = append(layouts[dir], scrollParts(s))

				strip := size - s.Content.Size().Width
				if mobile || c.content.Width > size {
					strip = 0 // no strip, or not one that is the bar's
				}
				want := c.strip
				if mobile {
					want = 0
				}
				assert.InDelta(t, want, strip, 0.01, "%v: the strip reserved for the bar", dir)

				// The content stops overflowing: no bar, no strip, and the
				// content fills the Scroll again.
				rect.SetMinSize(geo.NewSize(c.content.Width, 50))
				s.Refresh()
				layouts[dir] = append(layouts[dir], scrollParts(s))
			}
			bellui.SetLayoutDirection(bellui.DirectionAuto)

			ltr, rtl := layouts[bellui.DirectionLTR], layouts[bellui.DirectionRTL]
			require.Len(t, ltr, 3)
			for step := range ltr {
				if step < 2 {
					require.Contains(t, ltr[step], "bar visual", "step %d: the vertical bar is shown", step)
				} else {
					require.NotContains(t, ltr[step], "bar visual", "step %d: the vertical bar is hidden", step)
				}
				for k, want := range ltr[step] {
					got, ok := rtl[step][k]
					if assert.True(t, ok, "step %d: %s is drawn right-to-left", step, k) {
						assert.InDelta(t, size-want[0]-want[1], got[0], 0.01, "step %d: %s is the left-to-right one mirrored", step, k)
						assert.InDelta(t, want[1], got[1], 0.01, "step %d: %s's width", step, k)
					}
				}
				assert.Len(t, rtl[step], len(ltr[step]), "step %d: the same parts are drawn", step)
				if step < 2 {
					assert.InDelta(t, 0, rtl[step]["bar area"][0], 0.01, "step %d: right-to-left, the bar is at the left edge", step)
				}
			}
		})
	}
}

// Only the strip a Scroll reserved for its bar moves the content right under
// right-to-left; content that is narrower for any other reason stays at the
// left edge. Table's header row and column are Scrolls that scroll in no
// direction and are never extended (ExtendBaseWidget), so a Resize sets
// their size without a layout, and their renderer is created later, by a
// walk, with the content still 0×0. An earlier version of #1496 took the
// difference between the widths for the strip and moved the headers off the
// table, which only the order of the widget package's tests showed.
func TestScrollContainer_RTL_UnreservedNarrowContentStays(t *testing.T) {
	for _, dir := range []ScrollDirection{ScrollNone, ScrollVerticalOnly, ScrollBoth} {
		rtlScroll(t, bellui.DirectionRTL)
		rect := canvas.NewRectangle(color.Black)
		rect.SetMinSize(geo.NewSize(10, 10))
		s := &Scroll{Direction: dir, Content: rect}
		s.Resize(geo.NewSize(100, 100))
		_, made := cache.CachedRenderer(s)
		require.False(t, made, "direction %d: no renderer before the walk", dir)
		cache.Renderer(s) // as a walk of the tree creates it
		assert.Equal(t, geo.Position{}, rect.Position(), "direction %d: the content is at the left edge", dir)
	}
}
