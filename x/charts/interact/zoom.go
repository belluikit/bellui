package interact

import "github.com/belluikit/bellui/x/charts/internal/mathutil"

// DataWindow represents the visible data range as a fraction [0, 1] of the full range.
type DataWindow struct {
	Start float64 // 0.0 = beginning of data
	End   float64 // 1.0 = end of data
}

// FullWindow returns a DataWindow showing all data.
func FullWindow() DataWindow {
	return DataWindow{Start: 0, End: 1}
}

// Span returns the size of the window.
func (w DataWindow) Span() float64 {
	return w.End - w.Start
}

// ZoomConfig configures data zoom behavior.
type ZoomConfig struct {
	Enabled         bool
	MinSpan         float64 // minimum window size (fraction), default 0.01
	MaxSpan         float64 // maximum window size (fraction), default 1.0
	ZoomSensitivity float64 // scroll wheel multiplier, default 0.05
}

// DefaultZoomConfig returns a ZoomConfig with sensible defaults.
func DefaultZoomConfig() ZoomConfig {
	return ZoomConfig{
		Enabled:         true,
		MinSpan:         0.01,
		MaxSpan:         1.0,
		ZoomSensitivity: 0.05,
	}
}

// ZoomState manages the current data window and zoom operations.
type ZoomState struct {
	Window       DataWindow
	Config       ZoomConfig
	FollowLatest bool // when true, snap window.End to 1.0 on data updates
}

// NewZoomState creates a ZoomState showing all data.
func NewZoomState(cfg ZoomConfig) *ZoomState {
	return &ZoomState{
		Window: FullWindow(),
		Config: cfg,
	}
}

// ZoomAt zooms in (delta > 0) or out (delta < 0) centered on a normalized position [0, 1].
// Returns true if the window changed.
func (z *ZoomState) ZoomAt(delta float64, centerNorm float64) bool {
	if !z.Config.Enabled {
		return false
	}
	return z.zoomToSpan(z.Window.Span()*(1-delta*z.Config.ZoomSensitivity), centerNorm)
}

// ZoomBy zooms by factor around centerNorm (0..1 across the chart): above 1
// zooms in, showing span/factor of the data, below 1 zooms out. Used for
// pinch gestures, whose scale is a factor rather than a scroll delta.
// Returns true if the window changed.
func (z *ZoomState) ZoomBy(factor float64, centerNorm float64) bool {
	if !z.Config.Enabled || factor <= 0 {
		return false
	}
	return z.zoomToSpan(z.Window.Span()/factor, centerNorm)
}

func (z *ZoomState) zoomToSpan(newSpan float64, centerNorm float64) bool {
	span := z.Window.Span()
	newSpan = mathutil.Clamp(newSpan, z.Config.MinSpan, z.Config.MaxSpan)

	// Use epsilon comparison to avoid floating-point precision trapping
	// the span at a boundary where exact equality prevents any change.
	if newSpan == span || (newSpan-span > -1e-9 && newSpan-span < 1e-9) {
		return false
	}

	// Compute the center in data-window space
	center := z.Window.Start + centerNorm*span

	// Rebuild window around center
	z.Window.Start = center - newSpan*centerNorm
	z.Window.End = center + newSpan*(1-centerNorm)

	z.clamp()
	return true
}

// Pan shifts the window by a normalized delta (positive = right).
// Returns true if the window changed.
func (z *ZoomState) Pan(deltaNorm float64) bool {
	if !z.Config.Enabled {
		return false
	}

	oldStart := z.Window.Start
	z.Window.Start += deltaNorm
	z.Window.End += deltaNorm
	z.clamp()

	return z.Window.Start != oldStart
}

// SetWindow sets the data window directly.
func (z *ZoomState) SetWindow(start, end float64) {
	z.Window.Start = start
	z.Window.End = end
	z.clamp()
}

// SnapToLatest shifts the window so End = 1.0, preserving the current span.
// Used by FollowLatest mode after new data arrives.
func (z *ZoomState) SnapToLatest() {
	span := z.Window.Span()
	z.Window.End = 1.0
	z.Window.Start = 1.0 - span
	if z.Window.Start < 0 {
		z.Window.Start = 0
	}
}

// IsZoomed returns true if the window is not showing all data.
func (z *ZoomState) IsZoomed() bool {
	return z.Window.Start > 0.001 || z.Window.End < 0.999
}

func (z *ZoomState) clamp() {
	span := z.Window.Span()
	if z.Window.Start < 0 {
		z.Window.Start = 0
		z.Window.End = span
	}
	if z.Window.End > 1 {
		z.Window.End = 1
		z.Window.Start = 1 - span
	}
	if z.Window.Start < 0 {
		z.Window.Start = 0
	}
}
