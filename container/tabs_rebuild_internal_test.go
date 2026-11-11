package container

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/layoutgen"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Layout or Refresh with the same tabs keeps the bar's buttons container and
// its buttons, so the render tree gets no new nodes (#1059).
func TestAppTabs_SameTabsKeepTabBar(t *testing.T) {
	test.NewTempApp(t)
	tabs := NewAppTabs(
		NewTabItemWithIcon("One", theme.HomeIcon(), widget.NewLabel("1")),
		NewTabItem("Two", widget.NewLabel("2")),
		NewTabItem("Three", widget.NewLabel("3")),
	)
	w := test.NewTempWindow(t, tabs)
	w.Resize(geo.NewSize(400, 300))
	r := cache.Renderer(tabs).(*appTabsRenderer)
	buttons := r.bar.Objects[0].(*bellui.Container)
	objects := append([]core.CanvasObject(nil), buttons.Objects...)
	require.Len(t, objects, 3)

	for range 3 {
		tabs.Refresh()
		r.Layout(tabs.Size())
	}
	assert.Same(t, buttons, r.bar.Objects[0])
	assert.Equal(t, objects, buttons.Objects)
}

// A Layout with the same tabs refreshes no tab button. The selected tab's
// themed icon is rewrapped in a new primary resource on every refresh, so it
// staying the same object shows the button was not refreshed (#1059).
func TestAppTabs_LayoutRefreshesNoUnchangedButton(t *testing.T) {
	test.NewTempApp(t)
	tabs := NewAppTabs(
		NewTabItemWithIcon("One", theme.HomeIcon(), widget.NewLabel("1")),
		NewTabItem("Two", widget.NewLabel("2")),
	)
	w := test.NewTempWindow(t, tabs)
	w.Resize(geo.NewSize(400, 300))
	r := cache.Renderer(tabs).(*appTabsRenderer)
	button := r.bar.Objects[0].(*bellui.Container).Objects[0].(*tabButton)
	icon := cache.Renderer(button).(*tabButtonRenderer).icon
	res := icon.Resource

	for range 3 {
		r.Layout(tabs.Size())
	}
	assert.Same(t, res, icon.Resource)

	// An explicit Refresh still restyles every button: ThemeOverride relies
	// on it to apply a new theme to the subtree.
	tabs.Refresh()
	assert.NotSame(t, res, icon.Resource)

	// A change still reaches the button, without an explicit Refresh.
	tabs.Items[0].Text = "Uno"
	r.Layout(tabs.Size())
	assert.Equal(t, "Uno", cache.Renderer(button).(*tabButtonRenderer).label.Text)
}

// With tabs in the overflow menu, a Layout at the same size shows and hides
// nothing: each Show or Hide costs the next frame a full layout walk (#1059).
func TestAppTabs_OverflowLayoutTogglesNothing(t *testing.T) {
	test.NewTempApp(t)
	var items []*TabItem
	for _, s := range []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot", "Golf"} {
		items = append(items, NewTabItem(s, widget.NewLabel(s)))
	}
	tabs := NewAppTabs(items...)
	w := test.NewTempWindow(t, tabs)
	w.Resize(geo.NewSize(250, 300))
	r := cache.Renderer(tabs).(*appTabsRenderer)
	buttons := r.bar.Objects[0].(*bellui.Container)
	shown := len(buttons.Objects)
	require.Less(t, shown, len(items), "some tabs overflow")
	require.Greater(t, shown, 0)
	require.True(t, r.action.Visible())

	epoch := layoutgen.Epoch()
	for range 3 {
		r.Layout(tabs.Size())
	}
	assert.Equal(t, epoch, layoutgen.Epoch())
	assert.Same(t, buttons, r.bar.Objects[0])
	assert.Len(t, buttons.Objects, shown)
	assert.True(t, r.action.Visible())
}

// DocTabs keeps its buttons and refreshes none of them on a Layout with the
// same tabs, as AppTabs does (#1059).
func TestDocTabs_LayoutRefreshesNoUnchangedButton(t *testing.T) {
	test.NewTempApp(t)
	tabs := NewDocTabs(
		NewTabItemWithIcon("One", theme.HomeIcon(), widget.NewLabel("1")),
		NewTabItem("Two", widget.NewLabel("2")),
	)
	tabs.CreateTab = func() *TabItem { return NewTabItem("New", widget.NewLabel("new")) }
	w := test.NewTempWindow(t, tabs)
	w.Resize(geo.NewSize(400, 300))
	r := cache.Renderer(tabs).(*docTabsRenderer)
	buttons := r.scroller.Content.(*bellui.Container)
	objects := append([]core.CanvasObject(nil), buttons.Objects...)
	button := buttons.Objects[0].(*tabButton)
	icon := cache.Renderer(button).(*tabButtonRenderer).icon
	res := icon.Resource
	epoch := layoutgen.Epoch()

	for range 3 {
		r.Layout(tabs.Size())
	}
	assert.Same(t, res, icon.Resource)
	assert.Equal(t, objects, buttons.Objects)
	assert.Equal(t, epoch, layoutgen.Epoch())

	tabs.Items[0].Text = "Uno"
	r.Layout(tabs.Size())
	assert.Equal(t, "Uno", cache.Renderer(button).(*tabButtonRenderer).label.Text)
}
