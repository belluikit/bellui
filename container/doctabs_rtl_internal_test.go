package container

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// The DocTabs indicator covers the visible part of the selected tab, in both
// directions. In a right-to-left layout the create and overflow buttons are
// mirrored to the leading edge, so the scrolling tab strip starts after them
// (#1078). The expected span comes from where the driver places the tab
// button and the scroller, not from the renderer's own arithmetic.
func TestDocTabs_IndicatorCoversSelectedTab(t *testing.T) {
	for _, dir := range []bellui.Direction{bellui.DirectionLTR, bellui.DirectionRTL} {
		for _, n := range []int{2, 12} {
			t.Run(fmt.Sprintf("%v/%d tabs", dirName(dir), n), func(t *testing.T) {
				tabs, r := docTabsIn(t, dir, n)
				assertDocIndicator(t, tabs, r, "first tab")

				if n > 2 {
					tabs.SelectIndex(5) // scrolls it into view
					assertDocIndicator(t, tabs, r, "a middle tab")

					// Scrolled so half the selected tab is cut off at either
					// end of the strip: the indicator covers only the half
					// that shows.
					b := r.scroller.Content.(*bellui.Container).Objects[5]
					x, w, view := b.Position().X, b.Size().Width, r.scroller.Size().Width
					r.scroller.ScrollToOffset(geo.NewPos(x+w/2, 0))
					assertDocIndicator(t, tabs, r, "cut at the start of the strip")
					r.scroller.ScrollToOffset(geo.NewPos(x+w/2-view, 0))
					assertDocIndicator(t, tabs, r, "cut at the end of the strip")
				}
			})
		}
	}
}

func dirName(d bellui.Direction) string {
	if d == bellui.DirectionRTL {
		return "rtl"
	}
	return "ltr"
}

func docTabsIn(t *testing.T, dir bellui.Direction, n int) (*DocTabs, *docTabsRenderer) {
	t.Helper()
	test.NewTempApp(t)
	bellui.SetLayoutDirection(dir)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	items := make([]*TabItem, n)
	for i := range items {
		items[i] = NewTabItem("Tab with a long title", widget.NewLabel("x"))
	}
	tabs := NewDocTabs(items...)
	w := test.NewWindow(tabs)
	t.Cleanup(w.Close)
	w.SetPadded(false)
	w.Resize(geo.NewSize(400, 300))
	r := cache.Renderer(tabs).(*docTabsRenderer)
	if dir == bellui.DirectionRTL {
		require.Greater(t, r.scroller.Position().X, float32(0), "the create and overflow buttons come before the strip")
	}
	return tabs, r
}

func assertDocIndicator(t *testing.T, tabs *DocTabs, r *docTabsRenderer, what string) {
	t.Helper()
	d := bellui.CurrentApp().Driver()
	origin := d.AbsolutePositionForObject(tabs).X
	b := r.scroller.Content.(*bellui.Container).Objects[tabs.SelectedIndex()]
	tabStart := d.AbsolutePositionForObject(b).X - origin
	viewStart := d.AbsolutePositionForObject(r.scroller).X - origin
	start := geo.Max(tabStart, viewStart)
	end := geo.Min(tabStart+b.Size().Width, viewStart+r.scroller.Size().Width)
	require.Greater(t, end, start, "%s: the selected tab is in view", what)

	assert.InDelta(t, start, r.indicator.Position().X, 0.01, "%s: indicator start", what)
	assert.InDelta(t, end-start, r.indicator.Size().Width, 0.01, "%s: indicator width", what)
}
