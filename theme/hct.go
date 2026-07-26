package theme

import (
	"image/color"
	"math"
)

// HCT represents a color in the HCT (Hue-Chroma-Tone) color space,
// based on CAM16. Tone is perceptually uniform lightness (0=black, 100=white).
type HCT struct {
	Hue, Chroma, Tone float64
}

// TonalPalette generates colors at specific tone levels for a given hue and chroma.
type TonalPalette struct {
	hue, chroma float64
}

// CorePalettes holds the 6 tonal palettes that define an MD3 color scheme.
type CorePalettes struct {
	Primary, Secondary, Tertiary, Error, Neutral, NeutralVariant *TonalPalette
}

// NewTonalPalette creates a tonal palette for the given hue and chroma.
func NewTonalPalette(hue, chroma float64) *TonalPalette {
	return &TonalPalette{hue: hue, chroma: chroma}
}

// Tone returns the NRGBA color at the given tone level (0–100).
func (tp *TonalPalette) Tone(tone int) color.NRGBA {
	hct := HCT{Hue: tp.hue, Chroma: tp.chroma, Tone: float64(tone)}
	return hct.ToColor()
}

// CorePalettesFromPrimary generates all 6 MD3 palettes from a primary color.
func CorePalettesFromPrimary(primary color.NRGBA) *CorePalettes {
	hct := HCTFromColor(primary)
	return &CorePalettes{
		Primary:        NewTonalPalette(hct.Hue, math.Max(hct.Chroma, 48)),
		Secondary:      NewTonalPalette(hct.Hue, hct.Chroma/3),
		Tertiary:       NewTonalPalette(math.Mod(hct.Hue+60, 360), hct.Chroma/2),
		Error:          NewTonalPalette(25, 84),
		Neutral:        NewTonalPalette(hct.Hue, 4),
		NeutralVariant: NewTonalPalette(hct.Hue, 8),
	}
}

// HCTFromColor converts an NRGBA color to HCT.
func HCTFromColor(c color.NRGBA) HCT {
	r := srgbToLinear(float64(c.R) / 255)
	g := srgbToLinear(float64(c.G) / 255)
	b := srgbToLinear(float64(c.B) / 255)

	x, y, z := linearRGBToXYZ(r, g, b)
	tone := yToTone(y)
	hue, chroma := xyzToHueChroma(x, y, z)

	return HCT{Hue: hue, Chroma: chroma, Tone: tone}
}

// ToColor converts HCT to NRGBA, finding the sRGB color that best matches.
func (h HCT) ToColor() color.NRGBA {
	if h.Chroma < 1 || h.Tone <= 0 || h.Tone >= 100 {
		// Achromatic or extreme tone: just use Y-based gray.
		gray := toneToGray(h.Tone)
		return color.NRGBA{R: gray, G: gray, B: gray, A: 255}
	}

	// Binary search for the right chroma that produces a color
	// in sRGB gamut at the target hue and tone.
	hueRad := h.Hue * math.Pi / 180
	low, high := 0.0, h.Chroma
	bestR, bestG, bestB := 0.0, 0.0, 0.0

	for iter := 0; iter < 50; iter++ {
		mid := (low + high) / 2
		r, g, b := hctToLinearRGB(h.Hue, mid, h.Tone, hueRad)

		if r < -0.001 || r > 1.001 || g < -0.001 || g > 1.001 || b < -0.001 || b > 1.001 {
			high = mid // out of gamut, reduce chroma
		} else {
			low = mid
			bestR, bestG, bestB = r, g, b
		}
	}

	return color.NRGBA{
		R: clampByte(linearToSrgb(bestR) * 255),
		G: clampByte(linearToSrgb(bestG) * 255),
		B: clampByte(linearToSrgb(bestB) * 255),
		A: 255,
	}
}

// --- sRGB <-> Linear ---

func srgbToLinear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func linearToSrgb(c float64) float64 {
	if c <= 0.0031308 {
		return c * 12.92
	}
	return 1.055*math.Pow(c, 1.0/2.4) - 0.055
}

// --- Linear RGB <-> XYZ (D65) ---

func linearRGBToXYZ(r, g, b float64) (x, y, z float64) {
	x = 0.4124564*r + 0.3575761*g + 0.1804375*b
	y = 0.2126729*r + 0.7151522*g + 0.0721750*b
	z = 0.0193339*r + 0.1191920*g + 0.9503041*b
	return
}

func xyzToLinearRGB(x, y, z float64) (r, g, b float64) {
	r = 3.2404542*x - 1.5371385*y - 0.4985314*z
	g = -0.9692660*x + 1.8760108*y + 0.0415560*z
	b = 0.0556434*x - 0.2040259*y + 1.0572252*z
	return
}

// --- Tone <-> Y (CIE lightness) ---

func yToTone(y float64) float64 {
	// L* from CIE LAB
	if y <= 0.008856 {
		return y * 903.3
	}
	return 116*math.Pow(y, 1.0/3.0) - 16
}

func toneToY(tone float64) float64 {
	if tone <= 8 {
		return tone / 903.3
	}
	return math.Pow((tone+16)/116, 3)
}

func toneToGray(tone float64) uint8 {
	y := toneToY(tone)
	return clampByte(linearToSrgb(y) * 255)
}

// --- Hue/Chroma extraction (simplified CAM16-like) ---

func xyzToHueChroma(x, y, z float64) (hue, chroma float64) {
	// Simplified: use CIE LAB a*, b* for hue/chroma
	// This is not full CAM16 but produces perceptually reasonable results.
	fy := labF(y)
	fx := labF(x / 0.95047)
	fz := labF(z / 1.08883)

	a := 500 * (fx - fy)
	b := 200 * (fy - fz)

	chroma = math.Sqrt(a*a + b*b)
	hue = math.Atan2(b, a) * 180 / math.Pi
	if hue < 0 {
		hue += 360
	}
	return
}

func labF(t float64) float64 {
	if t > 0.008856 {
		return math.Pow(t, 1.0/3.0)
	}
	return (903.3*t + 16) / 116
}

// --- HCT to linear RGB (inverse: given H, C, T, find linear RGB) ---

func hctToLinearRGB(hue, chroma, tone, hueRad float64) (r, g, b float64) {
	// Convert tone to Y
	y := toneToY(tone)

	// Convert hue + chroma to LAB a*, b*
	a := chroma * math.Cos(hueRad)
	bLab := chroma * math.Sin(hueRad)

	// LAB to XYZ
	fy := labF(y)
	fx := a/500 + fy
	fz := fy - bLab/200

	xr := labFInv(fx) * 0.95047
	zr := labFInv(fz) * 1.08883

	r, g, b = xyzToLinearRGB(xr, y, zr)
	return
}

func labFInv(t float64) float64 {
	t3 := t * t * t
	if t3 > 0.008856 {
		return t3
	}
	return (116*t - 16) / 903.3
}

// --- Utilities ---

func clampByte(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(math.Round(v))
}
