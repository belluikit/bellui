package common

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/gpu"
)

// noteReplaced records that a Set*Tree call is replacing old with a tree
// rooted at next, for the next full walk to release what old held (#1075).
// ReleaseDropped records the same for content the app dropped (#1083).
func (c *Canvas) noteReplaced(old *renderCacheTree, next core.CanvasObject) {
	if old != nil && old.root != nil && old.root.obj != nil && old.root.obj != next {
		c.replaced = true
	}
}

// ReleaseDropped asks the next frame's walk to release what the caches and
// the painter hold for content the app took out of the canvas, as it does for
// replaced content (bellui.Release, #1083). It marks the canvas dirty so that
// frame comes. Call it on the main goroutine.
func (c *Canvas) ReleaseDropped() {
	c.replaced = true
	c.minSizeFull.Store(true)
	c.SetDirty()
}

// releaseReplaced runs after a full walk, once the offscreen sweep has run.
// If a tree was replaced since the last full walk, it releases what the caches
// hold for the objects that walk left out of the trees: their canvas-map
// entries and renderers, which a Clean drops only after a paint a minute
// later. An idle app makes no such paint, so a replaced screen stayed, with
// everything its handlers close over (#1075).
//
// What the trees still hold keeps its renderer: an object the new content took
// over from the old, a hidden one, and one under a hidden object or an
// unselected tab. Content the app puts back later gets new renderers, as it
// does today once it has been away for cache.ValidDuration.
func (c *Canvas) releaseReplaced() {
	if !c.replaced {
		return
	}
	c.replaced = false
	c.unfocusLeft()
	gone := cache.ReleaseUnkept(c.impl, c.inTrees)
	if c.painter == nil {
		return
	}
	if p, ok := c.painter.(gpu.BetweenFramesPainter); !ok || !p.CanFreeNow() {
		c.freeUnkept = true
		return
	}
	c.freeGPU(gone)
}

// unfocusLeft unfocuses the content's focused object if the walk left it out
// of the trees. The focus manager kept it, and with it the content it was
// dropped with, and sent it the keys typed after (#1083). A replaced tree
// gets a new focus manager, but content dropped from a container does not.
func (c *Canvas) unfocusLeft() {
	mgr := c.contentFocusMgr
	if mgr == nil {
		return
	}
	obj, ok := mgr.Focused().(core.CanvasObject)
	if !ok || c.inTrees(obj) {
		return
	}
	if mgr.Focus(nil) && mgr == c.focusManager() && c.OnUnfocus != nil {
		c.OnUnfocus()
	}
}

// freeGPU frees the textures and RepaintBoundary targets of the objects that
// left the canvas: gone, which the caches just let go of, and every object
// with a texture the last full walk left out of the trees, which is the leaf
// it draws, such as an image. The texture cache otherwise frees a texture at
// the first frame a minute after it was last drawn, and keeps the object, an
// image's decoded pixels with it, until then; an idle app draws no such frame
// (#1082).
//
// A painter that can free outside a frame, as WebGPU can, is asked in the
// walk. Otherwise the next FreeDirtyTextures, at the start of a frame, where
// GL has its context, frees the textures, which hold their objects until
// then anyway. The canvas keeps no list of them for it, as that list would
// hold the widgets the caches just let go of.
func (c *Canvas) freeGPU(gone []core.CanvasObject) {
	cache.RangeTexturesFor(c.impl, func(obj core.CanvasObject) {
		if !c.inTrees(obj) {
			c.painter.Free(obj)
		}
	})
	bp, ok := c.painter.(gpu.BoundaryPainter)
	if !ok {
		return
	}
	for _, obj := range gone {
		bp.FreeBoundaryFBO(obj)
	}
}

// freeUnkeptTextures frees, at the start of a frame, the textures replaced
// content left to a painter that could not free them in the walk (freeGPU).
func (c *Canvas) freeUnkeptTextures() {
	if !c.freeUnkept {
		return
	}
	c.freeUnkept = false
	c.freeGPU(nil)
}

// inTrees reports whether the last full walk found obj in the trees: it placed
// it, or skipped it as hidden, or recorded it under a hidden object.
func (c *Canvas) inTrees(obj core.CanvasObject) bool {
	if _, ok := c.nodeMap.Load(obj); ok {
		return true
	}
	if _, ok := c.hiddenUnder.Load(obj); ok {
		return true
	}
	_, ok := c.offscreen.Load(obj)
	return ok
}
