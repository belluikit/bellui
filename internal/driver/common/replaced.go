package common

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/cache"
)

// noteReplaced records that a Set*Tree call is replacing old with a tree
// rooted at next, for the next full walk to release what old held (#1075).
func (c *Canvas) noteReplaced(old *renderCacheTree, next core.CanvasObject) {
	if old != nil && old.root != nil && old.root.obj != nil && old.root.obj != next {
		c.replaced = true
	}
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
	cache.ReleaseUnkept(c.impl, c.inTrees)
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
