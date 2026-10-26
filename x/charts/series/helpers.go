package series

import (
	"image/color"
	"math"

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

// backgroundLabelColor is the colour of a label drawn on the plot background:
// ctx.TextColor, or fallback when the renderer runs without a chart to set it.
func backgroundLabelColor(ctx RenderContext, fallback color.Color) color.Color {
	if ctx.TextColor != nil {
		return ctx.TextColor
	}
	return fallback
}

// labelColorOn returns black or white, whichever contrasts more with fill,
// for a label drawn on a coloured shape: a white label on an amber or teal
// tile was faint (#343).
func labelColorOn(fill color.Color) color.Color {
	if fill == nil {
		return color.White
	}
	r, g, b, _ := fill.RGBA()
	lin := func(c uint32) float64 {
		v := float64(c) / 0xffff
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	// Relative luminance; black text has the higher contrast ratio above
	// about 0.179, where (L+0.05)/0.05 overtakes 1.05/(L+0.05).
	if 0.2126*lin(r)+0.7152*lin(g)+0.0722*lin(b) > 0.179 {
		return color.Black
	}
	return color.White
}
