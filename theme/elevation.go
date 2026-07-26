package theme

import (
	"image/color"

	col "github.com/belluikit/bellui/internal/color"
)

// MD3 elevation tint opacities (dark theme only).
// Level 0: 0%, Level 1: 5%, Level 2: 8%, Level 3: 11%, Level 4: 12%, Level 5: 14%
var elevationTintOpacity = [6]float64{0, 0.05, 0.08, 0.11, 0.12, 0.14}

// SurfaceWithElevation returns a surface color tinted with the primary color
// at the opacity appropriate for the given MD3 elevation level (0–5).
// In dark themes, elevated surfaces get a subtle primary color overlay.
// In light themes, the surface is returned unchanged (MD3 uses shadow, not tint, for light).
func SurfaceWithElevation(surface, primary color.Color, level int) color.Color {
	if level <= 0 || level > 5 {
		return surface
	}
	opacity := elevationTintOpacity[level]
	return blendSurfaceTint(surface, primary, opacity)
}

func blendSurfaceTint(surface, tint color.Color, opacity float64) color.Color {
	sr, sg, sb, sa := col.ToNRGBA(surface)
	tr, tg, tb, _ := col.ToNRGBA(tint)

	return color.NRGBA{
		R: uint8(float64(sr) + opacity*(float64(tr)-float64(sr))),
		G: uint8(float64(sg) + opacity*(float64(tg)-float64(sg))),
		B: uint8(float64(sb) + opacity*(float64(tb)-float64(sb))),
		A: uint8(sa),
	}
}
