package common

import (
	"runtime"
	"testing"
	"weak"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// idleFrame does what a driver's frame does with c that a test can: drain the
// refreshes, walk, and take the nodes queued for a region, as the paint does.
func idleFrame(c *Canvas) {
	c.FreeDirtyTextures()
	c.EnsureMinSize()
	c.MarkFullRepaint()
}

// screenState stands for what an app's screen closes over in its handlers.
type screenState struct{ events int }

// replacedScreen builds a screen whose every handler closes over s: a list
// scrolled so that rows wait in its pool, a focused entry, a hidden button, and
// tabs with a selected and an unselected tab.
func replacedScreen(s *screenState) (core.CanvasObject, *widget.List, *widget.Entry, *container.AppTabs) {
	list := widget.NewList(
		func() int { return 100 },
		func() core.CanvasObject { return widget.NewButton("row", nil) },
		func(_ widget.ListItemID, o core.CanvasObject) {
			o.(*widget.Button).OnTapped = func() { s.events++ }
		},
	)
	list.OnSelected = func(widget.ListItemID) { s.events++ }
	entry := widget.NewEntry()
	entry.OnChanged = func(string) { s.events++ }
	hidden := widget.NewButton("hidden", func() { s.events++ })
	hidden.Hide()
	tabs := container.NewAppTabs(
		container.NewTabItem("One", widget.NewButton("one", func() { s.events++ })),
		container.NewTabItem("Two", widget.NewButton("two", func() { s.events++ })),
	)
	content := container.NewBorder(container.NewVBox(entry, hidden), nil, nil, nil,
		container.NewGridWithColumns(2, list, tabs))
	return content, list, entry, tabs
}

// TestReplacedContentIsReleased is #1075: content the app replaced is not held
// by the renderer and canvas caches once the frame that shows the new content
// has walked it, although no paint follows. They used to hold it until a Clean
// after a paint a minute later, which an idle app does not make.
func TestReplacedContentIsReleased(t *testing.T) {
	test.NewTempApp(t)
	s := &screenState{}
	content, list, entry, tabs := replacedScreen(s)
	c := walkedCanvas(content)
	c.SetPainter(&stubPainter{})
	list.ScrollToBottom()
	idleFrame(c)
	c.Focus(entry)
	tabs.SelectIndex(1)
	c.Refresh(tabs)
	idleFrame(c)
	_, rendered := cache.CachedRenderer(entry)
	require.True(t, rendered, "precondition: the screen has renderers")

	next := container.NewVBox(widget.NewLabel("next"))
	c.impl.(*markingCanvas).SetContent(next)
	c.SetContentTreeAndFocusMgr(next)
	idleFrame(c) // the frame that shows next, and none after it

	_, rendered = cache.CachedRenderer(entry)
	assert.False(t, rendered, "the replaced entry keeps its renderer")
	assert.Nil(t, cache.GetCanvasForObject(entry), "the replaced entry keeps its canvas")
	w := weak.Make(s)
	s, content, list, entry, tabs = nil, nil, nil, nil, nil
	runtime.GC()
	runtime.GC()
	assert.Nil(t, w.Value(), "the replaced screen is still reachable")
	runtime.KeepAlive(c) // or the canvas, and all it holds, is collected with it
}

// TestReplacedContentKeepsWhatIsStillShown: what the new content took over
// from the old keeps its renderer and canvas, and so does an unselected tab's
// content and a hidden object under it.
func TestReplacedContentKeepsWhatIsStillShown(t *testing.T) {
	test.NewTempApp(t)
	kept := widget.NewEntry()
	hidden := widget.NewButton("hidden", nil)
	hidden.Hide()
	away := widget.NewButton("away", nil)
	tabs := container.NewAppTabs(
		container.NewTabItem("One", widget.NewLabel("one")),
		container.NewTabItem("Two", container.NewVBox(away, hidden)),
	)
	old := container.NewVBox(kept, tabs, widget.NewLabel("dropped"))
	c := walkedCanvas(old)
	tabs.SelectIndex(1)
	c.Refresh(tabs)
	c.EnsureMinSize()
	tabs.SelectIndex(0)
	c.Refresh(tabs)
	c.EnsureMinSize()

	next := container.NewVBox(kept, tabs)
	c.impl.(*markingCanvas).SetContent(next)
	c.SetContentTreeAndFocusMgr(next)
	c.EnsureMinSize()

	for _, wid := range []core.Widget{kept, tabs, away} {
		_, ok := cache.CachedRenderer(wid)
		assert.True(t, ok, "%T lost its renderer although still in the tree", wid)
		assert.NotNil(t, cache.GetCanvasForObject(wid), "%T lost its canvas although still in the tree", wid)
	}
}

// TestReplacedContentPutBackBeforeAWalk: content replaced and put back before
// any walk is in the trees that walk finds, and keeps its renderers.
func TestReplacedContentPutBackBeforeAWalk(t *testing.T) {
	test.NewTempApp(t)
	entry := widget.NewEntry()
	old := container.NewVBox(entry)
	c := walkedCanvas(old)

	next := container.NewVBox(widget.NewLabel("next"))
	c.SetContentTreeAndFocusMgr(next)
	c.SetContentTreeAndFocusMgr(old)
	c.EnsureMinSize()

	_, ok := cache.CachedRenderer(entry)
	assert.True(t, ok, "content put back lost its renderer")
}
