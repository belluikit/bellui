package common

import (
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/driver"
)

// cullMargin is how far outside a clip-owner's bounds the paint of its
// descendants may reach without the clip-owner's own clip stopping it: box
// shadows, which are drawn with the stencil off. It covers the elevation
// levels widgets use inside content (TestCullMarginCoversShadows); a
// clip-owner's own shadow is measured instead (shadowReach).
const cullMargin = 32

// CullOffClip is the paint walk's CullFunc: it culls a clip-owner (a scroll,
// a ClipToBounds container, a rounded clip such as a Card) whose bounds,
// widened by cullMargin and by its own shadow, miss the clip. A clip-owner's
// descendants draw inside its bounds, so nothing of the subtree can show. An
// object that owns no clip may draw outside its bounds, and is never culled.
func CullOffClip(obj core.CanvasObject, pos, clipPos geo.Position, clipSize geo.Size) bool {
	size := obj.Size()
	if !outside(pos, size, cullMargin, clipPos, clipSize) {
		return false
	}
	if !driver.IsClip(obj) {
		if rounded, _ := driver.IsRoundedClip(obj); !rounded {
			return false
		}
	}
	return outside(pos, size, cullMargin+shadowReach(obj), clipPos, clipSize)
}

// cullOffClip is CullOffClip for node's object, answering its renderer
// questions from the node while neither a refresh nor a theme change has made
// them stale.
func (r *RenderCacheNode) cullOffClip(pos, clipPos geo.Position, clipSize geo.Size) bool {
	size := r.obj.Size()
	if !outside(pos, size, cullMargin, clipPos, clipSize) {
		return false
	}
	clip, reach := r.clipInfo()
	return clip && outside(pos, size, cullMargin+reach, clipPos, clipSize)
}

// clipInfo is whether node's object owns a clip and, if so, its shadow reach,
// measured once per theme generation and per refresh.
func (r *RenderCacheNode) clipInfo() (bool, float32) {
	gen := cache.ThemeCacheGeneration() + 1
	was := r.cullGen.Load()
	if was == gen {
		return r.cullClip, r.cullReach
	}
	clip := driver.IsClip(r.obj)
	if !clip {
		clip, _ = driver.IsRoundedClip(r.obj)
	}
	reach := float32(0)
	if clip {
		reach = shadowReach(r.obj)
	}
	r.cullClip, r.cullReach = clip, reach
	// A refresh clearing the generation meanwhile leaves it clear, so the next
	// visit measures again.
	r.cullGen.CompareAndSwap(was, gen)
	return clip, reach
}

// PaintClips is the clips a paint walk sets up for node's object: none for a
// clip-owner whose children the walk culled, as they would constrain nothing
// (#481). Each one costs the painter a stencil or scissor change.
func PaintClips(node *RenderCacheNode) driver.Clips {
	if node.paintCulled {
		return driver.Clips{}
	}
	return driver.ClipsOf(node.obj)
}

// outside reports whether the rect (pos, size) widened by m misses the clip.
func outside(pos geo.Position, size geo.Size, m float32, clipPos geo.Position, clipSize geo.Size) bool {
	return pos.X+size.Width+m <= clipPos.X || pos.X-m >= clipPos.X+clipSize.Width ||
		pos.Y+size.Height+m <= clipPos.Y || pos.Y-m >= clipPos.Y+clipSize.Height
}

// shadowReach is how far the box shadows among a widget's own renderer objects
// reach outside it.
func shadowReach(obj core.CanvasObject) float32 {
	w, ok := obj.(core.Widget)
	if !ok {
		return 0
	}
	r, ok := cache.CachedRenderer(w)
	if !ok {
		return 0
	}
	reach := float32(0)
	for _, o := range r.Objects() {
		if s, ok := o.(*canvas.BoxShadow); ok {
			reach = max(reach, s.BlurRadius*2+max(abs32(s.OffsetX), abs32(s.OffsetY)))
		}
	}
	return reach
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
