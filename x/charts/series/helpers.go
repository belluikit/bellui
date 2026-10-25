package series

import (
	"image/color"

	"github.com/belluikit/bellui/x/charts/coord"
)

// ResolveColor returns override if non-nil, otherwise fallback.
func ResolveColor(override, fallback color.Color) color.Color {
	if override != nil {
		return override
	}
	return fallback
}

// ResolveFloat32 returns value if positive, otherwise defaultVal.
func ResolveFloat32(value, defaultVal float32) float32 {
	if value > 0 {
		return value
	}
	return defaultVal
}

// categoryWidthOf is the room a bar, candle, box or pictorial symbol has: the
// plot width shared by the data points that fall inside the plot. Unzoomed
// that is every point, so it is the plot width over the point count, as it
// always was; zoomed in, fewer points share the width and the shapes widen
// with it. They used to keep their unzoomed width (#343).
func categoryWidthOf(cs coord.System, n int) float32 {
	b := cs.Bounds()
	inside := 0
	for i := 0; i < n; i++ {
		if x := cs.DataToPixel(float64(i), 0).X; x >= b.X-0.5 && x <= b.X+b.W+0.5 {
			inside++
		}
	}
	if inside == 0 {
		inside = n
	}
	return b.W / float32(inside)
}
