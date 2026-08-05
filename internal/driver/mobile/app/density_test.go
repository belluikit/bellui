package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAndroidDPIForDensity(t *testing.T) {
	for _, tt := range []struct {
		name    string
		density int32
		dpi     int
		usable  bool
	}{
		{"unset", 0, 160, true}, // ACONFIGURATION_DENSITY_DEFAULT
		{"ldpi", 120, 120, true},
		{"mdpi", 160, 160, true},
		{"tvdpi", 213, 213, true},
		{"hdpi", 240, 240, true},
		{"xhdpi", 320, 320, true},
		{"xxhdpi", 480, 480, true},
		{"xxxhdpi", 640, 640, true},
		// Off-bucket densities are real hardware, taken as reported (#272).
		{"OPD2403", 356, 356, true},
		{"pixel", 420, 420, true},
		{"440dpi panel", 440, 440, true},
		// The sentinels are "no density", not a 65534/65535dpi screen.
		{"any", 0xfffe, 160, false},
		{"none", 0xffff, 160, false},
		{"negative", -1, 160, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dpi, usable := androidDPIForDensity(tt.density)
			assert.Equal(t, tt.dpi, dpi)
			assert.Equal(t, tt.usable, usable)
		})
	}
}
