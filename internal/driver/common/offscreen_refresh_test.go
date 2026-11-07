package common

import (
	"image/color"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// TestRefreshInUnselectedTabDrawsNoFrame is #1061 (player#389): an object on
// a tab that is not selected keeps refreshing on its own clock, and each
// refresh used to mark the window dirty, so the window drew frames nobody
// could see. Once a walk has seen the tab hidden, its objects' refreshes ask
// for no frame. Selecting the tab again does, and its objects are counted
// again. The test app routes no widget refresh to c, so the refreshes a
// widget would make are made on c.
func TestRefreshInUnselectedTabDrawsNoFrame(t *testing.T) {
	test.NewTempApp(t)
	glow := canvas.NewRectangle(color.White)
	label := widget.NewLabel("cover")
	tabs := container.NewAppTabs(
		container.NewTabItem("Player", container.NewStack(glow, label)),
		container.NewTabItem("Library", widget.NewLabel("songs")),
	)
	c := walkedCanvas(tabs)
	c.SetPainter(&recordingPainter{})

	c.CheckDirtyAndClear()
	c.Refresh(glow)
	require.True(t, c.IsDirty(), "a refresh on the selected tab must draw")
	c.FreeDirtyTextures()

	tabs.SelectIndex(1)
	c.Refresh(tabs)
	c.EnsureMinSize() // the walk of the next frame sees the tab hidden
	c.FreeDirtyTextures()
	c.CheckDirtyAndClear()

	for range 10 {
		c.Refresh(glow)
		c.Refresh(label)
	}
	assert.False(t, c.IsDirty(), "a refresh on an unselected tab asked for a frame")
	assert.Zero(t, c.FreeDirtyTextures(), "the mobile driver draws when this is above 0")

	tabs.SelectIndex(0)
	c.Refresh(tabs)
	assert.True(t, c.IsDirty(), "selecting the tab again must draw")
	c.EnsureMinSize()
	c.CheckDirtyAndClear()
	c.Refresh(glow)
	assert.True(t, c.IsDirty(), "a refresh on the tab selected again must draw")
}

// TestRefreshUnderHiddenContainer is the same for an ancestor's Hide and
// Show, and for an ancestor shown by its Hidden field and refreshed, which
// asks for a frame without Show.
func TestRefreshUnderHiddenContainer(t *testing.T) {
	test.NewTempApp(t)
	glow := canvas.NewRectangle(color.White)
	inner := container.NewStack(glow)
	box := container.NewVBox(container.NewPadded(inner), widget.NewLabel("other"))
	c := walkedCanvas(box)
	c.SetPainter(&recordingPainter{})

	inner.Hide()
	c.Refresh(inner)
	c.EnsureMinSize()
	c.CheckDirtyAndClear()
	c.Refresh(glow)
	assert.False(t, c.IsDirty(), "a refresh under a hidden container asked for a frame")

	inner.Show()
	c.Refresh(inner)
	assert.True(t, c.IsDirty(), "showing the container must draw")
	c.CheckDirtyAndClear()
	c.Refresh(glow) // before any walk has placed it again
	assert.True(t, c.IsDirty(), "a refresh under the container shown again must draw")

	inner.Hide()
	c.Refresh(inner)
	c.EnsureMinSize()
	c.CheckDirtyAndClear()
	inner.Hidden = false
	c.Refresh(inner)
	assert.True(t, c.IsDirty(), "a container shown by its Hidden field and refreshed must draw")
}

// TestHiddenTabShownAgainIsNotStale is #1061's second half: what an object
// on a hidden tab changed while it drew no frame is drawn when the tab is
// selected again, as the frame that selection asks for frees the textures
// refreshed in the meantime.
func TestHiddenTabShownAgainIsNotStale(t *testing.T) {
	test.NewTempApp(t)
	glow := canvas.NewRectangle(color.White)
	tabs := container.NewAppTabs(
		container.NewTabItem("Player", container.NewStack(glow)),
		container.NewTabItem("Library", widget.NewLabel("songs")),
	)
	c := walkedCanvas(tabs)
	p := &recordingPainter{}
	c.SetPainter(p)

	tabs.SelectIndex(1)
	c.Refresh(tabs)
	c.EnsureMinSize()
	c.FreeDirtyTextures()
	c.CheckDirtyAndClear()
	p.freed = nil

	glow.FillColor = color.Black
	c.Refresh(glow)
	require.False(t, c.IsDirty())

	tabs.SelectIndex(0)
	c.Refresh(tabs)
	require.True(t, c.IsDirty())
	c.FreeDirtyTextures()
	assert.Contains(t, p.freed, glow, "the frame showing the tab drew the glow from its old texture")
	c.EnsureMinSize()
	_, ok := c.nodeMap.Load(glow)
	assert.True(t, ok, "the walk of the frame showing the tab did not place the glow")
}

// walkedCanvas is a canvas over content that a min-size pass has walked, as
// the first frame does.
func walkedCanvas(content core.CanvasObject) *Canvas {
	content.Resize(geo.NewSize(360, 200))
	c := &Canvas{}
	impl := &markingCanvas{sizedCanvas: sizedCanvas{Canvas: test.NewCanvas()}, c: c}
	impl.SetContent(content)
	c.Initialize(impl, func() {})
	c.SetContentTreeAndFocusMgr(content)
	c.EnsureMinSize()
	return c
}

// TestRefreshOfObjectSwappedBackDraws pins why only hidden objects count: a
// renderer that swaps a visible child out and back, refreshing only the child
// it shows, must draw. An object out of the tree is not known to be out of
// sight, as a new object refreshed before any walk places it is out of the
// tree too.
func TestRefreshOfObjectSwappedBackDraws(t *testing.T) {
	test.NewTempApp(t)
	a, b := canvas.NewRectangle(color.White), canvas.NewRectangle(color.Black)
	box := container.NewStack(a)
	c := walkedCanvas(container.NewVBox(box))
	c.SetPainter(&recordingPainter{})

	box.Objects = []core.CanvasObject{b}
	c.Refresh(box)
	c.EnsureMinSize()
	_, ok := c.nodeMap.Load(a)
	require.False(t, ok, "a left the tree")
	c.CheckDirtyAndClear()

	box.Objects = []core.CanvasObject{a}
	c.Refresh(a)
	assert.True(t, c.IsDirty(), "the refresh of a, shown again, asked for no frame")
}

// TestOffscreenLetsGoOfWhatLeftForGood: content that left the tree hidden, as
// a closed tab's does, is not held once nothing refreshes it.
func TestOffscreenLetsGoOfWhatLeftForGood(t *testing.T) {
	test.NewTempApp(t)
	glow := canvas.NewRectangle(color.White)
	tabs := container.NewAppTabs(
		container.NewTabItem("Player", container.NewStack(glow)),
		container.NewTabItem("Library", widget.NewLabel("songs")),
	)
	c := walkedCanvas(tabs)
	tabs.SelectIndex(1)
	c.Refresh(tabs)
	c.EnsureMinSize()
	_, ok := c.offscreen.Load(glow)
	require.True(t, ok)

	at, _ := c.offscreen.Load(glow)
	at.seen.Store(time.Now().Add(-offscreenKeep - time.Second).UnixNano())
	c.WalkTrees(nil, nil)
	_, ok = c.offscreen.Load(glow)
	assert.False(t, ok, "nothing found the glow for offscreenKeep, and it is still held")
}

// TestInSight is what bellui.IsOnScreen asks the drivers' canvases (#1061):
// placed by the last walk, and nothing above it hidden since. A tab switch or
// an ancestor's Hide answers false at once, before the next walk; showing
// answers true from the walk that places the object again.
func TestInSight(t *testing.T) {
	test.NewTempApp(t)
	glow := canvas.NewRectangle(color.White)
	inner := container.NewStack(glow)
	tabs := container.NewAppTabs(
		container.NewTabItem("Player", container.NewVBox(inner)),
		container.NewTabItem("Library", widget.NewLabel("songs")),
	)
	c := walkedCanvas(tabs)
	assert.True(t, c.InSight(glow))
	assert.False(t, c.InSight(canvas.NewRectangle(color.White)), "an object no walk placed")

	tabs.SelectIndex(1)
	assert.False(t, c.InSight(glow), "out of sight on a tab not selected, before the next walk")
	c.Refresh(tabs)
	c.EnsureMinSize()
	assert.False(t, c.InSight(glow), "out of sight on a tab not selected")

	tabs.SelectIndex(0)
	c.Refresh(tabs)
	c.EnsureMinSize()
	assert.True(t, c.InSight(glow), "in sight on the tab selected again")

	inner.Hide()
	assert.False(t, c.InSight(glow), "out of sight under a container hidden, before the next walk")
	inner.Show()
	c.Refresh(inner)
	c.EnsureMinSize()
	assert.True(t, c.InSight(glow), "in sight under the container shown again")
}
