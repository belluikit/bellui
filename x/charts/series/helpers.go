package series

import "image/color"

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
