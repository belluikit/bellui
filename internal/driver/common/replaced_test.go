package common

import (
	"image"
	"runtime"
	"testing"
	"weak"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/gpu"
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
// The pointer keeps it out of the tiny allocator, which packs small objects
// without pointers into one block: a weak pointer to one of them is not
// cleared while another in its block lives, so whether it was depended on
// what the tests before had allocated.
type screenState struct {
	events int
	_      *int
}

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

// gpuPainter frees as a GPU painter does, recording what it freed: Free drops
// an object's texture from the cache. canFree is what it answers CanFreeNow;
// glPainter wraps one to answer nothing, as GL does not implement it.
type gpuPainter struct {
	stubPainter
	canFree    bool
	freed      []core.CanvasObject
	boundaries []core.CanvasObject
}

func (p *gpuPainter) Free(obj core.CanvasObject) {
	if _, ok := cache.GetTexture(obj); ok {
		p.freed = append(p.freed, obj)
		cache.DeleteTexture(obj)
	}
}

func (p *gpuPainter) CanFreeNow() bool { return p.canFree }

func (*gpuPainter) EnsureBoundaryFBO(core.CanvasObject, int, int, geo.Size) gpu.BoundaryFBO {
	return nil
}
func (*gpuPainter) BindBoundaryFBO(gpu.BoundaryFBO)                             {}
func (*gpuPainter) UnbindBoundaryFBO(int, int)                                  {}
func (*gpuPainter) BlitBoundaryTexture(gpu.BoundaryFBO, geo.Position, geo.Size) {}
func (*gpuPainter) MarkBoundaryDirty(core.CanvasObject)                         {}
func (*gpuPainter) IsBoundaryClean(core.CanvasObject) bool                      { return false }
func (*gpuPainter) InvalidateAllBoundaries()                                    {}

func (p *gpuPainter) FreeBoundaryFBO(obj core.CanvasObject) {
	if _, ok := obj.(*container.RepaintBoundary); ok {
		p.boundaries = append(p.boundaries, obj)
	}
}

var (
	_ gpu.BetweenFramesPainter = (*gpuPainter)(nil)
	_ gpu.BoundaryPainter      = (*gpuPainter)(nil)
)

// glPainter is a gpuPainter that cannot free outside a frame, as GL cannot.
type glPainter struct{ p *gpuPainter }

func (g glPainter) Init()                                                  {}
func (g glPainter) Capture(core.Canvas) image.Image                        { return nil }
func (g glPainter) Clear()                                                 {}
func (g glPainter) Free(obj core.CanvasObject)                             { g.p.Free(obj) }
func (g glPainter) Paint(core.CanvasObject, geo.Position, geo.Size)        {}
func (g glPainter) SetFrameBufferScale(float32)                            {}
func (g glPainter) SetOutputSize(int, int)                                 {}
func (g glPainter) StartClipping(geo.Position, geo.Size)                   {}
func (g glPainter) StopClipping()                                          {}
func (g glPainter) StartRoundedClipping(geo.Position, geo.Size, float32)   {}
func (g glPainter) StopRoundedClipping()                                   {}
func (g glPainter) StartPathClipping(*canvas.Path, geo.Position, geo.Size) {}
func (g glPainter) StopPathClipping()                                      {}
func (g glPainter) SetGroupOpacity(float32)                                {}
func (g glPainter) ClearGroupOpacity()                                     {}
func (g glPainter) SetTransform(geo.Matrix)                                {}

// texturedScreen is a screen with an image the painter holds a texture for,
// under a RepaintBoundary, and an image the next screen takes over.
func texturedScreen(c *Canvas, kept *canvas.Image) (*canvas.Image, *container.RepaintBoundary) {
	img := canvas.NewImageFromImage(image.NewNRGBA(image.Rect(0, 0, 512, 512)))
	img.SetMinSize(geo.NewSize(50, 50))
	b := container.NewRepaintBoundary(container.NewVBox(img, kept))
	content := container.NewVBox(b)
	c.impl.(*markingCanvas).SetContent(content)
	c.SetContentTreeAndFocusMgr(content)
	idleFrame(c)
	// The handle's value does not matter: the cache holds an entry for each,
	// and TextureType is a struct on GL mobile and wasm builds.
	cache.SetTexture(img, cache.NoTexture, c.impl)
	cache.SetTexture(kept, cache.NoTexture, c.impl)
	return img, b
}

// TestReplacedContentFreesTextures is #1082: the walk that lets go of
// replaced content frees its textures and RepaintBoundary targets too, when
// the painter can free outside a frame, so no frame has to follow. The
// texture cache held each until a frame a minute after it was last drawn,
// and the image it was keyed by with it.
func TestReplacedContentFreesTextures(t *testing.T) {
	test.NewTempApp(t)
	c := walkedCanvas(container.NewVBox())
	p := &gpuPainter{canFree: true}
	c.SetPainter(p)
	kept := canvas.NewImageFromImage(image.NewNRGBA(image.Rect(0, 0, 8, 8)))
	img, b := texturedScreen(c, kept)

	next := container.NewVBox(kept)
	c.impl.(*markingCanvas).SetContent(next)
	c.SetContentTreeAndFocusMgr(next)
	c.EnsureMinSize() // the walk of the frame that shows next, before its FreeDirtyTextures

	assert.Contains(t, p.freed, core.CanvasObject(img), "the replaced image's texture was not freed in the walk")
	assert.NotContains(t, p.freed, core.CanvasObject(kept), "the texture of an image still shown was freed")
	assert.Contains(t, p.boundaries, core.CanvasObject(b), "the replaced boundary's target was not freed in the walk")
	_, ok := cache.GetTexture(kept)
	assert.True(t, ok, "the image still shown lost its texture")

	p.freed, p.boundaries = nil, nil
	w := weak.Make(img)
	img, b = nil, nil
	runtime.GC()
	runtime.GC()
	assert.Nil(t, w.Value(), "the replaced image is still reachable")
	runtime.KeepAlive(c)
}

// TestReplacedContentTexturesWaitForAFrame: a painter that cannot free outside
// a frame, as GL cannot, frees the replaced content's textures at the start
// of the next frame, not in the walk.
func TestReplacedContentTexturesWaitForAFrame(t *testing.T) {
	test.NewTempApp(t)
	c := walkedCanvas(container.NewVBox())
	p := &gpuPainter{}
	c.SetPainter(glPainter{p})
	kept := canvas.NewImageFromImage(image.NewNRGBA(image.Rect(0, 0, 8, 8)))
	img, _ := texturedScreen(c, kept)

	next := container.NewVBox(kept)
	c.impl.(*markingCanvas).SetContent(next)
	c.SetContentTreeAndFocusMgr(next)
	c.EnsureMinSize()
	assert.Empty(t, p.freed, "a painter that cannot free outside a frame was asked to")

	c.FreeDirtyTextures()
	assert.Equal(t, []core.CanvasObject{img}, p.freed)
	p.freed = nil
	c.FreeDirtyTextures()
	assert.Empty(t, p.freed, "a second frame freed again")
}

// TestReplacedContentTexturesWaitForAnOpenFrame: a painter that can free
// outside a frame but has one open is asked at the next frame instead.
func TestReplacedContentTexturesWaitForAnOpenFrame(t *testing.T) {
	test.NewTempApp(t)
	c := walkedCanvas(container.NewVBox())
	p := &gpuPainter{canFree: true}
	c.SetPainter(p)
	kept := canvas.NewImageFromImage(image.NewNRGBA(image.Rect(0, 0, 8, 8)))
	img, _ := texturedScreen(c, kept)

	p.canFree = false // a frame is open
	next := container.NewVBox(kept)
	c.impl.(*markingCanvas).SetContent(next)
	c.SetContentTreeAndFocusMgr(next)
	c.EnsureMinSize()
	assert.Empty(t, p.freed)

	p.canFree = true
	c.FreeDirtyTextures()
	assert.Equal(t, []core.CanvasObject{img}, p.freed)
}
