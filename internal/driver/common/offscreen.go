package common

import (
	"sync/atomic"
	"time"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
)

// offscreenKeep is how long an object under a hidden object stays recorded
// after a walk or its own refresh last found it there. An object under a
// hidden object still in the tree is found again by every full walk, which
// runs at least every fullMinSizeEvery; one that left the tree with it, as an
// unselected tab's content does, only by its refreshes. So what is out of the
// tree for good is let go once nothing refreshes it.
const offscreenKeep = time.Minute

// offscreenAt is the hidden object that obj sits under, obj itself if it is
// that object, and when a walk or a refresh last found it there.
type offscreenAt struct {
	top  core.CanvasObject
	seen atomic.Int64 // UnixNano
}

// noteOffscreen records that top is hidden and out of sight, and so every
// object under it: their refreshes then ask for no frame (isOffscreen). A walk
// calls it for a hidden object it skips, and for a hidden object whose node
// leaves the tree, which is how an unselected tab's content goes: AppTabs
// hides it and stops listing it. The subtree is recorded at most once per
// full-walk generation, which also records objects added under top since.
// Only renderers that exist are read: an object under a widget never rendered
// is not recorded, and its refreshes still ask for a frame.
func (c *Canvas) noteOffscreen(top core.CanvasObject) {
	if gen, ok := c.offscreenTops.Load(top); ok && gen == c.walkGen {
		return // storing over a key allocates (mapNode)
	}
	c.offscreenTops.Store(top, c.walkGen)
	now := time.Now().UnixNano()
	var under *offscreenAt
	driver.WalkCompleteObjectTree(top, func(obj core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
		if at, ok := c.offscreen.Load(obj); ok && at.top == top {
			at.seen.Store(now)
			return false
		}
		if under == nil {
			under = &offscreenAt{top: top}
			under.seen.Store(now)
		}
		c.offscreen.Store(obj, under)
		return false
	}, nil)
}

// isOffscreen reports whether obj is out of sight under a hidden object, so
// that its refresh needs no frame (#1061): the hidden object it was recorded
// under is still hidden, and no walk has placed obj since, elsewhere. A
// refresh that shows the object, by Show or by its Hidden field, still draws,
// as the object is then visible. Safe from any goroutine.
func (c *Canvas) isOffscreen(obj core.CanvasObject) bool {
	at, ok := c.offscreen.Load(obj)
	if !ok || at.top.Visible() {
		return false
	}
	if _, placed := c.nodeMap.Load(obj); placed {
		return false
	}
	at.seen.Store(time.Now().UnixNano())
	return true
}

// sweepOffscreen runs after the full walk numbered gen. It forgets the hidden
// objects that walk did not skip, so the next walk to find one records its
// subtree again, and drops the objects whose hidden object is shown or that
// nothing found for offscreenKeep.
func (c *Canvas) sweepOffscreen(gen uint64) {
	c.offscreenTops.Range(func(top core.CanvasObject, at uint64) bool {
		if at != gen {
			c.offscreenTops.Delete(top)
		}
		return true
	})
	stale := time.Now().Add(-offscreenKeep).UnixNano()
	c.offscreen.Range(func(obj core.CanvasObject, at *offscreenAt) bool {
		if at.seen.Load() < stale || at.top.Visible() {
			c.offscreen.Delete(obj)
		}
		return true
	})
}

// InSight reports whether the last walk placed obj in this canvas' trees, and
// it and every object above it are still visible: one hidden since, by Hide or
// a tab switch, is out of sight at once, without waiting for the next walk
// (bellui.IsOnScreen). Call it on the main goroutine, which walks the trees.
func (c *Canvas) InSight(obj core.CanvasObject) bool {
	node, ok := c.nodeMap.Load(obj)
	if !ok {
		return false
	}
	for n := node; n != nil; n = n.parent {
		if !n.obj.Visible() {
			return false
		}
	}
	return true
}
