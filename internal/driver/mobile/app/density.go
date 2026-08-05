package app

// Density values from the NDK's <android/configuration.h>, mirrored here so
// the mapping below is ordinary Go that builds — and is tested — on every
// platform: android.go is cgo behind a `android` build tag, so no CI leg
// compiles it (#272). It asserts at compile time that these still match the
// C constants.
const (
	densityDefault = 0      // ACONFIGURATION_DENSITY_DEFAULT: no density in the config
	densityMedium  = 160    // ACONFIGURATION_DENSITY_MEDIUM: the dp baseline
	densityAny     = 0xfffe // ACONFIGURATION_DENSITY_ANY
	densityNone    = 0xffff // ACONFIGURATION_DENSITY_NONE
)

// androidDPIForDensity turns what AConfiguration_getDensity reports into the
// screen dpi, and says whether the report was a usable density at all.
//
// The reported value is the device's configured dpi (Configuration.densityDpi)
// and is not restricted to the classic bucket constants — 356, 420 or 440dpi
// panels are common — so any positive value below the sentinels is taken as-is
// rather than rounded or logged as "unknown"; the canvas scale is then derived
// from it arithmetically (androidScaleForDPI, #272). ACONFIGURATION_DENSITY_ANY
// (0xfffe) and _NONE (0xffff) are sentinels meaning "no density", not
// densities: both fall back to mdpi, i.e. scale 1.0 — what the old dpi-72
// fallback produced through the bucket table — and report usable == false so
// the caller can log them. ANY previously fell through as a 65534dpi screen.
func androidDPIForDensity(density int32) (dpi int, usable bool) {
	switch {
	case density > 0 && density < densityAny:
		return int(density), true
	case density == densityDefault: // density simply unset, not a bad report
		return densityMedium, true
	default: // the ANY/NONE sentinels, or a negative: not a density
		return densityMedium, false
	}
}
