package cache

import (
	"os"
	"sync/atomic"
	"time"

	"github.com/belluikit/bellui/core"
)

var (
	ValidDuration     = 1 * time.Minute
	cleanTaskInterval = ValidDuration / 2

	lastClean                     atomic.Value // stores time.Time
	skippedCleanWithCanvasRefresh atomic.Bool

	// CleanHooks are additional cleanup functions called during each Clean() cycle.
	// Used to allow packages like painter to register cache sweeps without circular imports.
	CleanHooks []func()

	// themeCacheGen is bumped every time the text/svg caches are reset for a theme
	// or font change (ResetThemeCaches). Consumers that hold their own measurement
	// memo derived from the global metrics cache read ThemeCacheGeneration to drop
	// it in lock-step, so a theme swap or a late RegisterFontFallback (which pairs
	// ClearFontCache with ResetThemeCaches) never serves stale sizes (pre-rebrand tracker #896).
	themeCacheGen atomic.Uint64

	// cachedNow is the clock sample, in Unix nanoseconds, that setAlive stamps
	// entries with. The paint walk keeps every widget alive every frame, so
	// reading the clock there cost about 15% of a frame's CPU (#458). Clean
	// takes a new sample, once per frame on desktop and the web.
	cachedNow atomic.Int64

	// testing purpose only
	timeNow = time.Now
)

func init() {
	lastClean.Store(time.Time{})
	cachedNow.Store(time.Now().UnixNano())
	if t, err := time.ParseDuration(os.Getenv("BELLUI_CACHE")); err == nil {
		ValidDuration = t
		cleanTaskInterval = ValidDuration / 2
	}
}

func getLastClean() time.Time {
	if v := lastClean.Load(); v != nil {
		return v.(time.Time)
	}
	return time.Time{}
}

// sampledNow returns the clock sample entries are currently stamped with.
func sampledNow() time.Time {
	return time.Unix(0, cachedNow.Load())
}

// Clean run cache clean task, it should be called on paint events.
func Clean(canvasRefreshed bool) {
	// Expire against the sample entries were stamped with, not the clock. The
	// mobile driver cleans only after a paint: after a long idle, that paint
	// stamps the widgets on screen with the old sample, and a cutoff taken
	// from the clock would destroy their renderers while they are shown.
	cutoff := sampledNow()
	now := timeNow()
	cachedNow.Store(now.UnixNano())
	last := getLastClean()
	// do not run clean task too fast
	if now.Sub(last) < 10*time.Second {
		if canvasRefreshed {
			skippedCleanWithCanvasRefresh.Store(true)
		}
		return
	}
	if skippedCleanWithCanvasRefresh.CompareAndSwap(true, false) {
		canvasRefreshed = true
	}
	if !canvasRefreshed && now.Sub(last) < cleanTaskInterval {
		return
	}
	destroyExpiredSvgs(cutoff)
	destroyExpiredFontMetrics(cutoff)
	if canvasRefreshed {
		// Destroy renderers on canvas refresh to avoid flickering screen.
		destroyExpiredRenderers(cutoff)
		// canvases cache should be invalidated only on canvas refresh, otherwise there wouldn't
		// be a way to recover them later
		destroyExpiredCanvases(cutoff)
	}
	for _, hook := range CleanHooks {
		hook()
	}
	lastClean.Store(timeNow())
}

// CleanCanvas performs a complete remove of all the objects that belong to the specified
// canvas. Usually used to free all objects from a closing window.
// Also sweeps any expired renderers to avoid GPU resource leaks from widgets whose
// canvas entries expired before the canvas was closed.
func CleanCanvas(canvas core.Canvas) {
	canvases.Range(func(obj core.CanvasObject, cinfo *canvasInfo) bool {
		if cinfo.canvas != canvas {
			return true
		}

		canvases.Delete(obj)

		wid, ok := obj.(core.Widget)
		if !ok {
			return true
		}
		rinfo, ok := renderers.LoadAndDelete(wid)
		if !ok {
			return true
		}
		rinfo.renderer.Destroy()
		overrides.Delete(wid)
		return true
	})

	// Sweep any expired renderers that lost their canvas association.
	destroyExpiredRenderers(sampledNow())
}

// ResetThemeCaches clears all the svg and text size cache maps
// minSizeEpoch counts direct min-size settings (SetMinSize), which refresh
// nothing, so no canvas hears of them otherwise.
var minSizeEpoch atomic.Uint64

// MinSizeSet records that an object's min size was set directly. The next
// min-size pass of every canvas asks every object again (#464): a pass that
// asks only refreshed objects would miss it.
func MinSizeSet() { minSizeEpoch.Add(1) }

// MinSizeEpoch returns the count MinSizeSet raises.
func MinSizeEpoch() uint64 { return minSizeEpoch.Load() }

func ResetThemeCaches() {
	svgs.Clear()
	fontSizeCache.Clear()
	themeCacheGen.Add(1)
}

// ThemeCacheGeneration returns a counter bumped each time the theme/font caches
// are reset (ResetThemeCaches). A widget-level measurement memo compares this
// against the generation it was built under and drops itself when it differs, so
// it stays consistent with the authoritative global metrics cache (pre-rebrand tracker #896).
func ThemeCacheGeneration() uint64 {
	return themeCacheGen.Load()
}

// destroyExpiredCanvases deletes objects from the canvases cache.
func destroyExpiredCanvases(now time.Time) {
	canvases.Range(func(obj core.CanvasObject, cinfo *canvasInfo) bool {
		if cinfo.isExpired(now) {
			canvases.Delete(obj)
		}
		return true
	})
}

// destroyExpiredRenderers deletes the renderer from the cache and calls
// renderer.Destroy()
func destroyExpiredRenderers(now time.Time) {
	renderers.Range(func(wid core.Widget, rinfo *rendererInfo) bool {
		if rinfo.isExpired(now) {
			rinfo.renderer.Destroy()
			overrides.Delete(wid)
			renderers.Delete(wid)
		}
		return true
	})
}

type expiringCache struct {
	// expires holds the expiry time as Unix nanoseconds. It is stored
	// atomically because setAlive (reached from MeasureText, possibly off the
	// paint thread) and the paint-thread cleaner access it concurrently; a
	// plain multi-word time.Time would tear under that race.
	expires atomic.Int64
}

// isExpired check if the cache data is expired.
func (c *expiringCache) isExpired(now time.Time) bool {
	return c.expires.Load() < now.UnixNano()
}

// setAlive updates expiration time.
func (c *expiringCache) setAlive() {
	c.expires.Store(cachedNow.Load() + ValidDuration.Nanoseconds())
}
