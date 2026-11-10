package container

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

// #1124: HideTabBar drops the bar, divider and indicator, and the selected
// content fills the whole widget; turning it off puts the bar back.
func TestAppTabs_HideTabBar(t *testing.T) {
	test.NewTempApp(t)

	c1 := widget.NewLabel("one")
	c2 := widget.NewLabel("two")
	tabs := NewAppTabs(NewTabItem("One", c1), NewTabItem("Two", c2))
	w := test.NewTempWindow(t, tabs)
	w.Resize(geo.NewSize(300, 200))
	size := tabs.Size()

	r := test.TempWidgetRenderer(t, tabs).(*appTabsRenderer)
	withBar := tabs.MinSize()
	assert.Greater(t, c1.Position().Y, float32(0), "with the bar, the content sits below it")
	assert.Contains(t, r.Objects(), r.bar)

	tabs.HideTabBar = true
	tabs.Refresh()

	assert.Equal(t, geo.NewPos(0, 0), c1.Position())
	assert.Equal(t, size, c1.Size(), "the selected content fills the widget")
	assert.False(t, c2.Visible())
	assert.Equal(t, []any{c1}, toAny(r.Objects()), "only the selected content is drawn")
	assert.Equal(t, c1.MinSize().Max(c2.MinSize()), tabs.MinSize())
	assert.Less(t, tabs.MinSize().Height, withBar.Height)

	// Selection still works and still fills the widget.
	tabs.SelectIndex(1)
	assert.Equal(t, geo.NewPos(0, 0), c2.Position())
	assert.Equal(t, size, c2.Size())
	assert.False(t, c1.Visible())
	assert.Equal(t, []any{c2}, toAny(r.Objects()))

	tabs.HideTabBar = false
	tabs.Refresh()
	assert.Greater(t, c2.Position().Y, float32(0), "the bar is back above the content")
	assert.Contains(t, r.Objects(), r.bar)
	assert.Equal(t, withBar, tabs.MinSize())
}

func toAny[T any](in []T) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}
