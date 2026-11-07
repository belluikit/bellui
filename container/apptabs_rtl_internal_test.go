package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// In a right-to-left layout the overflow button comes first and moves the tab
// buttons along by its width; the indicator must follow them.
func TestAppTabs_IndicatorRTLWithOverflow(t *testing.T) {
	test.NewTempApp(t)
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	items := make([]*TabItem, 12)
	for i := range items {
		items[i] = NewTabItem("Tab with a long title", widget.NewLabel("x"))
	}
	tabs := NewAppTabs(items...)
	w := test.NewWindow(tabs)
	defer w.Close()
	w.SetPadded(false)
	w.Resize(geo.NewSize(400, 300))
	r := cache.Renderer(tabs).(*appTabsRenderer)
	require.True(t, r.action.Visible(), "tabs overflow")
	buttons := r.bar.Objects[0]
	require.Greater(t, buttons.Position().X, float32(0), "the overflow button is before the tabs")

	first := buttons.(*bellui.Container).Objects[0]
	assert.Equal(t, r.bar.Position().X+buttons.Position().X+first.Position().X, r.indicator.Position().X)
	assert.Equal(t, first.Size().Width, r.indicator.Size().Width)
}
