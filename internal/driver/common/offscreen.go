package common

import (
	"slices"
	"sync/atomic"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
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
// that object, the object that hidden one was a child of when a walk found it
// (nil for a tree's root), and when a walk or a refresh last found it there.
// sweptGen and inTree are what the sweep after the full walk numbered
// sweptGen found of parent (inTree), kept so that it asks once per hidden
// object rather than once per object under it.
type offscreenAt struct {
	top, parent core.CanvasObject
	seen        atomic.Int64 // UnixNano
	sweptGen    uint64
	inTree      bool
}

// noteOffscreen records that top, a child of parent, is hidden and out of
// sight, and so every object under it: their refreshes then ask for no frame
// (isOffscreen), and once top is shown again they are in sight before a walk
// places them (InSight). A walk calls it for a hidden object it skips, and for
// a hidden object whose node leaves the tree, which is how an unselected tab's
// content goes: AppTabs hides it and stops listing it. The subtree is recorded
// at most once per full-walk generation, which also records objects added
// under top since. Only renderers that exist are read: an object under a
// widget never rendered is not recorded, and its refreshes still ask for a
// frame.
func (c *Canvas) noteOffscreen(top, parent core.CanvasObject) {
	if gen, ok := c.offscreenTops.Load(top); ok && gen == c.walkGen {
		return // storing over a key allocates (mapNode)
	}
	c.offscreenTops.Store(top, c.walkGen)
	now := time.Now().UnixNano()
	var under *offscreenAt
	driver.WalkCompleteObjectTree(top, func(obj core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
		if at, ok := c.offscreen.Load(obj); ok && at.top == top && at.parent == parent {
			at.seen.Store(now)
			return false
		}
		if under == nil {
			under = &offscreenAt{top: top, parent: parent}
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
// subtree again, and drops the objects whose hidden object is shown, that
// nothing found for offscreenKeep, or whose hidden object's parent left the
// trees (#1074). That last is how replaced content goes: the walk of the
// frame that shows the new content drops it, where waiting out offscreenKeep
// needs a walk a minute later, which an idle app does not make. All are asked
// before any is dropped, as whether one is in the trees can go through
// another's entry.
func (c *Canvas) sweepOffscreen(gen uint64) {
	c.offscreenTops.Range(func(top core.CanvasObject, at uint64) bool {
		if at != gen {
			c.offscreenTops.Delete(top)
		}
		return true
	})
	c.offscreen.Range(func(_ core.CanvasObject, at *offscreenAt) bool {
		c.offscreenInTree(at, gen, 0)
		return true
	})
	stale := time.Now().Add(-offscreenKeep).UnixNano()
	c.offscreen.Range(func(obj core.CanvasObject, at *offscreenAt) bool {
		if !at.inTree || at.seen.Load() < stale || at.top.Visible() {
			c.offscreen.Delete(obj)
		}
		return true
	})
}

// offscreenInTree reports whether at's hidden object is still a child of an
// object in this canvas' trees, as the full walk numbered gen left them: its
// parent is placed, or is itself recorded under a hidden object that is, as
// a hidden object inside an unselected tab's content is. A tree's root has no
// parent to ask, and is kept until offscreenKeep. depth stops a cycle, which
// only entries recorded at different times could make.
func (c *Canvas) offscreenInTree(at *offscreenAt, gen uint64, depth int) bool {
	if at.sweptGen == gen {
		return at.inTree
	}
	in := at.parent == nil
	if !in {
		if _, placed := c.nodeMap.Load(at.parent); placed {
			in = true
		} else if up, ok := c.offscreen.Load(at.parent); ok && up != at && depth < 64 {
			in = c.offscreenInTree(up, gen, depth+1)
		}
	}
	at.sweptGen, at.inTree = gen, in
	return in
}

// InSight reports whether obj is in sight in this canvas' trees: the last walk
// placed it, and it and every object above it are still visible. One hidden
// since, by Hide or a tab switch, is out of sight at once, without waiting for
// the next walk (bellui.IsOnScreen). So is one shown again at once, before the
// walk that places it (#1069): see shownAgain. Call it on the main goroutine,
// which walks the trees.
func (c *Canvas) InSight(obj core.CanvasObject) bool {
	node, ok := c.nodeMap.Load(obj)
	if !ok {
		return c.shownAgain(obj)
	}
	for n := node; n != nil; n = n.parent {
		if !n.obj.Visible() {
			return false
		}
	}
	return true
}

// shownAgain reports whether obj, which no walk has placed since it went out
// of sight under a hidden object, is in sight again although the walk that
// places it is still to come: that hidden object is shown, it is still a
// child of the object it was a child of, which is in sight, and obj is still
// under it with nothing between them hidden. That walk is the paint of the
// next frame, so without this a consumer asking on each tick skips its work
// for that frame, which then draws what it showed when it went out of sight
// (#1069). Only an object recorded out of sight is found: one added under the
// hidden object since the last walk that recorded it is in sight from the walk
// that places it. The cost is a walk of the hidden object's subtree, through
// the renderers that exist, for each question until that walk: one frame.
func (c *Canvas) shownAgain(obj core.CanvasObject) bool {
	at, ok := c.offscreen.Load(obj)
	if !ok || at.parent == nil || !at.top.Visible() || !c.InSight(at.parent) {
		return false
	}
	if !slices.Contains(childrenOf(at.parent), at.top) {
		return false // moved while hidden, or its parent stopped listing it
	}
	var path []core.CanvasObject
	found := false
	driver.WalkCompleteObjectTree(at.top, func(o core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
		path = append(path, o)
		if o != obj {
			return false
		}
		found = true
		for _, above := range path {
			if !above.Visible() {
				found = false
			}
		}
		return true
	}, func(core.CanvasObject, geo.Position, core.CanvasObject) {
		path = path[:len(path)-1]
	})
	return found
}

// childrenOf is what a walk visits below obj, from the renderers that exist.
func childrenOf(obj core.CanvasObject) []core.CanvasObject {
	switch o := obj.(type) {
	case *bellui.Container:
		return o.Objects
	case core.Widget:
		if r, _ := cache.CachedRenderer(o); r != nil {
			return r.Objects()
		}
	}
	return nil
}
