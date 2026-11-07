package common

import (
	"image/color"
	"runtime"
	"testing"
	"weak"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// TestOffscreenLetsGoOfReplacedContent is #1074: content replaced while
// something in it was hidden is not held by the offscreen map once the frame
// that shows the new content has walked it, although nothing draws after it.
// The map used to keep it until a walk a minute later, and an idle app makes
// no walk.
func TestOffscreenLetsGoOfReplacedContent(t *testing.T) {
	test.NewTempApp(t)
	glow := canvas.NewRectangle(color.White)
	inner := container.NewStack(glow)
	inner.Hide()
	tabbed := canvas.NewRectangle(color.White)
	tabs := container.NewAppTabs(
		container.NewTabItem("Player", container.NewStack(tabbed)),
		container.NewTabItem("Library", widget.NewLabel("songs")),
	)
	c := walkedCanvas(container.NewVBox(container.NewPadded(inner), tabs))
	tabs.SelectIndex(1)
	c.Refresh(tabs)
	c.EnsureMinSize()
	for _, obj := range []core.CanvasObject{glow, tabbed} {
		_, ok := c.offscreen.Load(obj)
		require.True(t, ok, "precondition: recorded out of sight")
	}

	next := container.NewVBox(canvas.NewRectangle(color.Black))
	c.impl.(*markingCanvas).SetContent(next)
	c.SetContentTreeAndFocusMgr(next)
	c.EnsureMinSize() // the frame that shows next, and none after it
	c.MarkFullRepaint()

	held := 0
	c.offscreen.Range(func(core.CanvasObject, *offscreenAt) bool {
		held++
		return true
	})
	assert.Zero(t, held, "the replaced content's entries are still in the offscreen map")
	w := weak.Make(glow)
	glow, inner = nil, nil
	runtime.GC()
	runtime.GC()
	assert.Nil(t, w.Value(), "the hidden object of the replaced content is still reachable")
	runtime.KeepAlive(c)
}

// TestOffscreenKeepsWhatIsStillInTheTree: an unselected tab's content, out of
// the tree but under tabs that are in it, stays recorded across walks, and so
// does a hidden object under it; so does a hidden object still in the tree.
func TestOffscreenKeepsWhatIsStillInTheTree(t *testing.T) {
	test.NewTempApp(t)
	glow := canvas.NewRectangle(color.White)
	inner := container.NewStack(glow)
	inner.Hide()
	tabbed := canvas.NewRectangle(color.White)
	deep := canvas.NewRectangle(color.White)
	deepBox := container.NewStack(deep)
	tabs := container.NewAppTabs(
		container.NewTabItem("Player", container.NewVBox(tabbed, deepBox)),
		container.NewTabItem("Library", widget.NewLabel("songs")),
	)
	c := walkedCanvas(container.NewVBox(container.NewPadded(inner), tabs))
	deepBox.Hide()
	c.Refresh(deepBox)
	c.EnsureMinSize()
	tabs.SelectIndex(1)
	c.Refresh(tabs)
	c.EnsureMinSize()

	for range 3 {
		c.WalkTrees(nil, nil)
	}
	for _, obj := range []core.CanvasObject{glow, tabbed, deep} {
		_, ok := c.offscreen.Load(obj)
		assert.True(t, ok, "dropped although still under the tree")
	}
	c.CheckDirtyAndClear()
	c.Refresh(tabbed)
	c.Refresh(deep)
	assert.False(t, c.IsDirty(), "a refresh on an unselected tab asked for a frame")
}
