package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// markingCanvas is the window's canvas as the drivers have it: one whose
// MarkMinSizeDirty reaches the common canvas, as BaseWidget.Refresh expects.
type markingCanvas struct {
	sizedCanvas
	c *Canvas
}

func (m *markingCanvas) MarkMinSizeDirty(obj core.CanvasObject) { m.c.MarkMinSizeDirty(obj) }

// TestShownObjectJoinsTreeThroughRefreshedWidget pins the assumption behind
// MarkMinSizeDirty's early return (#487, #490): an object refreshed while no
// walk has placed it gets its node from the walk of the refreshed widget's
// path, without a full walk. A progress bar at 0 hides its fill, whose node
// the walk then drops; the update that shows it again refreshes the fill
// before any walk has placed it. That is the tiles scene's ~200 refreshes a
// second of visible objects outside the tree.
func TestShownObjectJoinsTreeThroughRefreshedWidget(t *testing.T) {
	test.NewTempApp(t)
	bar := widget.NewProgressBar()
	objs := []core.CanvasObject{bar}
	for range 20 {
		objs = append(objs, widget.NewLabel("other"))
	}
	content := container.NewVBox(objs...)
	content.Resize(geo.NewSize(200, 800))
	c := &Canvas{}
	impl := &markingCanvas{sizedCanvas: sizedCanvas{Canvas: test.NewCanvas()}, c: c}
	impl.SetContent(content)
	c.Initialize(impl, func() {})
	c.SetContentTreeAndFocusMgr(content)

	c.EnsureMinSize()
	full := c.MinSizeVisits()
	for range 2 { // the first passes measure everything; then only changes
		c.EnsureMinSize()
	}
	require.Equal(t, 1, c.MinSizeVisits(), "the walk has not settled")
	fill := cache.Renderer(bar).Objects()[1]
	require.False(t, fill.Visible(), "a bar at 0 hides its fill")
	_, ok := c.nodeMap.Load(fill)
	require.False(t, ok, "a hidden object has no node")

	bar.SetValue(0.01)
	require.True(t, fill.Visible())
	c.EnsureMinSize()
	_, ok = c.nodeMap.Load(fill)
	assert.True(t, ok, "the walk of the bar's path did not place the fill it showed")
	assert.Less(t, c.MinSizeVisits(), full, "placing it took a full walk")
}
