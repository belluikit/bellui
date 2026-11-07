package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// dragTabs shows AppTabs with three tabs of different widths, unpadded at the
// window's origin, so canvas positions are the renderer's.
func dragTabs(t *testing.T, location TabLocation) (*AppTabs, *appTabsRenderer, core.Canvas) {
	t.Helper()
	test.NewTempApp(t)
	tabs := NewAppTabs(
		NewTabItem("A", widget.NewLabel("1")),
		NewTabItem("Longer tab", widget.NewLabel("2")),
		NewTabItem("Mid tab", widget.NewLabel("3")),
	)
	tabs.DragToSelect = true
	tabs.SetTabLocation(location)
	w := test.NewWindow(tabs)
	t.Cleanup(w.Close)
	w.SetPadded(false)
	w.Resize(geo.NewSize(400, 300))
	r := cache.Renderer(tabs).(*appTabsRenderer)
	require.Len(t, r.bar.Objects[0].(*bellui.Container).Objects, 3, "every tab fits in the bar")
	return tabs, r, w.Canvas()
}

// tabPoint returns the canvas point at the centre of tab button i.
func tabPoint(r *appTabsRenderer, i int) geo.Position {
	b := r.bar.Objects[0].(*bellui.Container).Objects[i]
	return r.bar.Position().Add(r.bar.Objects[0].Position()).Add(b.Position()).
		AddXY(b.Size().Width/2, b.Size().Height/2)
}

// indicatorCenter returns the centre of the indicator along the bar's axis.
func indicatorCenter(r *appTabsRenderer) float32 {
	if r.verticalBar() {
		return r.indicator.Position().Y + r.indicator.Size().Height/2
	}
	return r.indicator.Position().X + r.indicator.Size().Width/2
}

func along(r *appTabsRenderer, p geo.Position) float32 {
	if r.verticalBar() {
		return p.Y
	}
	return p.X
}

// assertIndicatorOnTab checks the indicator covers exactly tab button i.
func assertIndicatorOnTab(t *testing.T, r *appTabsRenderer, i int) {
	t.Helper()
	b := r.bar.Objects[0].(*bellui.Container).Objects[i]
	assert.InDelta(t, along(r, tabPoint(r, i)), indicatorCenter(r), 0.01, "indicator centred on tab %d", i)
	length := r.indicator.Size().Width
	want := b.Size().Width
	if r.verticalBar() {
		length, want = r.indicator.Size().Height, b.Size().Height
	}
	assert.InDelta(t, want, length, 0.01, "indicator as long as tab %d", i)
}

func TestAppTabs_DragToSelect_IndicatorFollowsAndSnaps(t *testing.T) {
	for name, loc := range map[string]TabLocation{
		"top": TabLocationTop, "bottom": TabLocationBottom,
		"leading": TabLocationLeading, "trailing": TabLocationTrailing,
	} {
		t.Run(name, func(t *testing.T) {
			tabs, r, c := dragTabs(t, loc)
			var selected []string
			tabs.OnSelected = func(ti *TabItem) { selected = append(selected, ti.Text) }

			from, to := tabPoint(r, 0), tabPoint(r, 2)
			delta := to.Subtract(from)
			const steps = 4
			test.DragSteps(c, from, delta.X, delta.Y, steps, func(step int) {
				f := float32(step) / steps
				finger := along(r, from) + (along(r, to)-along(r, from))*f
				assert.InDelta(t, finger, indicatorCenter(r), 0.01, "step %d: indicator centred on the pointer", step)
				assert.Equal(t, 0, tabs.SelectedIndex(), "step %d: selection waits for the release", step)
			})

			assert.Equal(t, 2, tabs.SelectedIndex())
			assert.Equal(t, []string{"Mid tab"}, selected, "OnSelected fires once, on release")
			assertIndicatorOnTab(t, r, 2)
		})
	}
}

func TestAppTabs_DragToSelect_LengthBlendsBetweenTabs(t *testing.T) {
	_, r, c := dragTabs(t, TabLocationTop)
	a, b := tabPoint(r, 0), tabPoint(r, 1)
	wa := r.bar.Objects[0].(*bellui.Container).Objects[0].Size().Width
	wb := r.bar.Objects[0].(*bellui.Container).Objects[1].Size().Width
	if wa == wb {
		t.Skip("the tabs are laid out in equal cells (mobile): no lengths to blend")
	}

	// Halfway between the two centres, the indicator is halfway between the widths.
	test.DragSteps(c, a, (b.X-a.X)/2, 0, 1, func(int) {
		assert.InDelta(t, (wa+wb)/2, r.indicator.Size().Width, 0.01)
	})
}

func TestAppTabs_DragToSelect_ReleaseNearSelectedReturns(t *testing.T) {
	tabs, r, c := dragTabs(t, TabLocationTop)
	tabs.SelectIndex(1)
	called := false
	tabs.OnSelected = func(*TabItem) { called = true }

	p := tabPoint(r, 1)
	test.DragSteps(c, p, 6, 0, 2, nil)

	assert.Equal(t, 1, tabs.SelectedIndex())
	assert.False(t, called, "no new selection")
	assertIndicatorOnTab(t, r, 1)
}

func TestAppTabs_DragToSelect_PastTheEndsClamps(t *testing.T) {
	tabs, r, c := dragTabs(t, TabLocationTop)
	p := tabPoint(r, 1)
	test.DragSteps(c, p, 1000, 0, 1, func(int) {
		assert.InDelta(t, tabPoint(r, 2).X, indicatorCenter(r), 0.01, "held at the last tab")
	})
	assert.Equal(t, 2, tabs.SelectedIndex())

	test.DragSteps(c, p, -1000, 0, 1, func(int) {
		assert.InDelta(t, tabPoint(r, 0).X, indicatorCenter(r), 0.01, "held at the first tab")
	})
	assert.Equal(t, 0, tabs.SelectedIndex())
}

func TestAppTabs_DragToSelect_SkipsDisabledTab(t *testing.T) {
	tabs, r, c := dragTabs(t, TabLocationTop)
	tabs.DisableIndex(1)

	// Released on the disabled middle tab, near its far edge: the last tab's
	// centre is nearer than the first's.
	mid := r.bar.Objects[0].(*bellui.Container).Objects[1]
	from := tabPoint(r, 0)
	to := tabPoint(r, 1).AddXY(mid.Size().Width/2-1, 0)
	require.Less(t, tabPoint(r, 2).X-to.X, to.X-from.X)
	test.DragSteps(c, from, to.X-from.X, 0, 2, nil)

	assert.Equal(t, 2, tabs.SelectedIndex(), "the disabled tab is passed over")
	assertIndicatorOnTab(t, r, 2)
}

func TestAppTabs_DragToSelect_TapStillSelects(t *testing.T) {
	tabs, r, c := dragTabs(t, TabLocationTop)
	test.TapCanvas(c, tabPoint(r, 1))
	assert.Equal(t, 1, tabs.SelectedIndex())
	assertIndicatorOnTab(t, r, 1)
}

func TestAppTabs_DragToSelect_OffByDefault(t *testing.T) {
	tabs, r, c := dragTabs(t, TabLocationTop)
	tabs.DragToSelect = false
	tabs.Refresh()

	assert.NotContains(t, r.Objects(), r.dragSurface)
	from, to := tabPoint(r, 0), tabPoint(r, 2)
	test.DragSteps(c, from, to.X-from.X, 0, 2, nil)
	assert.Equal(t, 0, tabs.SelectedIndex(), "a drag does nothing")

	assert.False(t, NewAppTabs().DragToSelect)
}

func TestAppTabs_DragToSelect_ClaimsDragAlongTheBar(t *testing.T) {
	_, r, _ := dragTabs(t, TabLocationTop)
	assert.True(t, r.dragSurface.ClaimsDrag(geo.NewDelta(5, 1)))
	assert.False(t, r.dragSurface.ClaimsDrag(geo.NewDelta(1, 5)), "a drag across the bar is left to a scroll")

	r.appTabs.location = TabLocationLeading // set directly: mobile moves a side bar in portrait
	assert.True(t, r.dragSurface.ClaimsDrag(geo.NewDelta(1, 5)))
	assert.False(t, r.dragSurface.ClaimsDrag(geo.NewDelta(5, 1)))
}

func TestAppTabs_DragToSelect_SurfaceCoversOnlyTheButtons(t *testing.T) {
	test.NewTempApp(t)
	items := make([]*TabItem, 12)
	for i := range items {
		items[i] = NewTabItem("Tab with a long title", widget.NewLabel("x"))
	}
	tabs := NewAppTabs(items...)
	tabs.DragToSelect = true
	w := test.NewWindow(tabs)
	defer w.Close()
	w.SetPadded(false)
	w.Resize(geo.NewSize(400, 300))
	r := cache.Renderer(tabs).(*appTabsRenderer)
	require.True(t, r.action.Visible(), "tabs overflow")

	buttons := r.bar.Objects[0]
	assert.Equal(t, buttons.Size(), r.dragSurface.Size())
	assert.Equal(t, r.bar.Position().Add(buttons.Position()), r.dragSurface.Position())
}

// In a right-to-left layout the first tab is on the right: a drag leftwards
// from it moves the indicator onto the later tabs.
func TestAppTabs_DragToSelect_RTL(t *testing.T) {
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })
	tabs, r, c := dragTabs(t, TabLocationTop)

	from, to := tabPoint(r, 0), tabPoint(r, 2)
	require.Less(t, to.X, from.X, "the last tab is on the left")
	test.DragSteps(c, from, (to.X-from.X)/2, 0, 1, func(int) {
		assert.InDelta(t, from.X+(to.X-from.X)/2, indicatorCenter(r), 0.01)
	})
	test.DragSteps(c, from, to.X-from.X, 0, 3, nil)

	assert.Equal(t, 2, tabs.SelectedIndex())
	assertIndicatorOnTab(t, r, 2)
}
