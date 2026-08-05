package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Device(t *testing.T) {
	dev := &device{}

	assert.Equal(t, true, dev.IsMobile())
	assert.Equal(t, false, dev.IsBrowser())
	assert.Equal(t, false, dev.HasKeyboard())
}

func TestAndroidScaleForDPI(t *testing.T) {
	// Bucket densities keep the scales the old lookup table produced.
	assert.Equal(t, float32(1), androidScaleForDPI(160))
	assert.Equal(t, float32(1.5), androidScaleForDPI(240))
	assert.Equal(t, float32(2), androidScaleForDPI(320))
	assert.Equal(t, float32(3), androidScaleForDPI(480))
	assert.Equal(t, float32(4), androidScaleForDPI(640))

	// tvdpi is the one bucket whose scale changes: the old ladder rounded 213
	// up to 1.5, DisplayMetrics says 1.33125 (#272).
	assert.InDelta(t, 1.33125, androidScaleForDPI(213), 0.0001)

	// Off-bucket densities derive arithmetically, matching
	// DisplayMetrics.density — the OPD2403 reports 356 (#272).
	assert.InDelta(t, 2.225, androidScaleForDPI(356), 0.0001)
	assert.InDelta(t, 2.625, androidScaleForDPI(420), 0.0001)
	assert.InDelta(t, 3.5, androidScaleForDPI(560), 0.0001)

	// At or below mdpi clamps to 1: no shipping ldpi hardware, and dpi 0
	// (no size event yet) must stay usable.
	assert.Equal(t, float32(1), androidScaleForDPI(0))
	assert.Equal(t, float32(1), androidScaleForDPI(120))
}
