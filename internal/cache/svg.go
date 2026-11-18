package cache

import (
	"image"
	"time"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/async"
)

var svgs async.Map[string, *svgInfo]

// RenderDeferredImage draws what a canvas.Image put off, at the image's size:
// the SVG it did not rasterize while it was not on screen (#1137), or a File
// or Resource set with no Refresh after it (#1298). It does nothing for any
// other object. The painters call it before they read an image's pixels;
// canvas sets it.
var RenderDeferredImage = func(core.CanvasObject) {}

// GetSvg gets svg image from cache if it exists.
func GetSvg(name string, o core.CanvasObject, w int, h int) *image.NRGBA {
	svginfo, ok := svgs.Load(overriddenName(name, o))
	if !ok || svginfo == nil {
		return nil
	}

	if svginfo.w != w || svginfo.h != h {
		return nil
	}

	svginfo.setAlive()
	return svginfo.pix
}

// SetSvg sets a svg into the cache map.
func SetSvg(name string, o core.CanvasObject, pix *image.NRGBA, w int, h int) {
	sinfo := &svgInfo{
		pix: pix,
		w:   w,
		h:   h,
	}
	sinfo.setAlive()
	svgs.Store(overriddenName(name, o), sinfo)
}

type svgInfo struct {
	expiringCache
	pix  *image.NRGBA
	w, h int
}

// destroyExpiredSvgs destroys expired svgs cache data.
func destroyExpiredSvgs(now time.Time) {
	svgs.Range(func(key string, sinfo *svgInfo) bool {
		if sinfo.isExpired(now) {
			svgs.Delete(key)
		}
		return true
	})
}

func overriddenName(name string, o core.CanvasObject) string {
	if o != nil { // for overridden themes get the cache key right
		if over, ok := overrides.Load(o); ok {
			return over.cacheID + name
		}
	}

	return name
}
