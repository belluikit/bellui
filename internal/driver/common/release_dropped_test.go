package common

import (
	"runtime"
	"testing"
	"weak"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// pagedCanvas is the MD3 gallery's shape: a slot whose child is the page,
// swapped by assigning Objects (gallery main.go, setPage). The page is
// replacedScreen, whose handlers close over s.
func pagedCanvas(t *testing.T, s *screenState) (*Canvas, *bellui.Container, *widget.Entry) {
	page, list, entry, tabs := replacedScreen(s)
	slot := container.NewStack(page)
	c := walkedCanvas(container.NewBorder(widget.NewLabel("nav"), nil, nil, nil, slot))
	c.SetPainter(&stubPainter{})
	list.ScrollToBottom()
	idleFrame(c)
	c.Focus(entry)
	tabs.SelectIndex(1)
	c.Refresh(tabs)
	idleFrame(c)
	_, rendered := cache.CachedRenderer(entry)
	require.True(t, rendered, "precondition: the page has renderers")
	return c, slot, entry
}

// setPage swaps the slot's page as the gallery does.
func setPage(c *Canvas, slot *bellui.Container, page core.CanvasObject) {
	slot.Objects = []core.CanvasObject{page}
	slot.Refresh()
	c.Refresh(slot)
}

// TestReleaseDroppedPage is #1083: a page the app swaps out of a container,
// and releases, is not held by the caches once the frame that shows the next
// page has walked, although no paint follows.
func TestReleaseDroppedPage(t *testing.T) {
	test.NewTempApp(t)
	s := &screenState{}
	c, slot, entry := pagedCanvas(t, s)

	setPage(c, slot, widget.NewLabel("next"))
	c.ReleaseDropped()
	assert.True(t, c.CheckDirtyAndClear(), "a release asks for no frame")
	idleFrame(c) // the frame that shows the next page, and none after it

	_, rendered := cache.CachedRenderer(entry)
	assert.False(t, rendered, "the dropped entry keeps its renderer")
	assert.Nil(t, cache.GetCanvasForObject(entry), "the dropped entry keeps its canvas")
	assert.Nil(t, c.Focused(), "the dropped entry keeps the focus")
	w := weak.Make(s)
	s, entry = nil, nil
	runtime.GC()
	runtime.GC()
	assert.Nil(t, w.Value(), "the dropped page is still reachable")
	runtime.KeepAlive(c)
}

// TestDroppedPageWithoutRelease pins the owner's call on #1083: there is no
// automatic trigger, so a page swapped out of a container without a release
// keeps its renderers until the caches expire them.
func TestDroppedPageWithoutRelease(t *testing.T) {
	test.NewTempApp(t)
	c, slot, entry := pagedCanvas(t, &screenState{})

	setPage(c, slot, widget.NewLabel("next"))
	idleFrame(c)

	_, rendered := cache.CachedRenderer(entry)
	assert.True(t, rendered, "a container change released its old child")
}

// TestReleasedPageShownAgain: a page released and put back gets new
// renderers and is drawn.
func TestReleasedPageShownAgain(t *testing.T) {
	test.NewTempApp(t)
	s := &screenState{}
	page, _, entry, _ := replacedScreen(s)
	slot := container.NewStack(page)
	c := walkedCanvas(slot)
	c.SetPainter(&stubPainter{})
	idleFrame(c)
	first, _ := cache.CachedRenderer(entry)

	setPage(c, slot, widget.NewLabel("next"))
	c.ReleaseDropped()
	idleFrame(c)
	setPage(c, slot, page)
	idleFrame(c)

	again, rendered := cache.CachedRenderer(entry)
	require.True(t, rendered, "the page put back has no renderer")
	assert.NotSame(t, first, again, "the released page kept its old renderer")
	assert.NotNil(t, cache.GetCanvasForObject(entry), "the page put back is on no canvas")
	_, placed := c.nodeMap.Load(core.CanvasObject(entry))
	assert.True(t, placed, "the page put back was not walked")
}

// TestListPoolKeepsRenderersAcrossScrolls: scrolling a list releases nothing,
// as no release is triggered by the rows it takes out of the tree to reuse.
// The renderer of a row scrolled away is the one it has when scrolled back.
func TestListPoolKeepsRenderersAcrossScrolls(t *testing.T) {
	test.NewTempApp(t)
	var rows []*widget.Button
	list := widget.NewList(
		func() int { return 200 },
		func() core.CanvasObject {
			b := widget.NewButton("row", nil)
			rows = append(rows, b)
			return b
		},
		func(widget.ListItemID, core.CanvasObject) {},
	)
	c := walkedCanvas(list)
	c.SetPainter(&stubPainter{})
	idleFrame(c)
	renderers := map[*widget.Button]core.WidgetRenderer{}
	for _, b := range rows {
		if r, ok := cache.CachedRenderer(b); ok {
			renderers[b] = r
		}
	}
	require.NotEmpty(t, renderers)

	for range 5 {
		list.ScrollToBottom()
		idleFrame(c)
		list.ScrollToTop()
		idleFrame(c)
	}

	for b, r := range renderers {
		now, ok := cache.CachedRenderer(b)
		assert.True(t, ok, "a pooled row lost its renderer to a scroll")
		assert.Same(t, r, now, "a pooled row got a new renderer from a scroll")
	}
}
