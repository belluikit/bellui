package mobile

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/internal/driver/mobile/event/size"
	"github.com/belluikit/bellui/lang"
)

type device struct {
	safeTop, safeLeft, safeBottom, safeRight int

	keyboardShown bool
}

//lint:file-ignore U1000 Var currentDPI is used in other files, but not here
var (
	currentOrientation size.Orientation
	currentDPI         float32
)

// Declare conformity with Device
var _ bellui.Device = (*device)(nil)

// androidScaleForDPI converts the density Android reports
// (Configuration.densityDpi, carried here as currentDPI) into the canvas
// scale the way the platform itself does: DisplayMetrics.density is
// densityDpi/160, so everything derived from dp — touch-target floors, text
// sizes — comes out physically correct on devices configured between the
// classic buckets (356, 420, 440dpi panels are common). The former lookup
// table rounded those to the nearest step: an OPD2403 reporting 356 ran at
// scale 2.0 instead of 2.225, ~10% small in physical units (#272). Bucket
// densities map exactly as before (240→1.5, 320→2, 480→3, 640→4). At or
// below mdpi the scale clamps to 1 — sub-1 scales served no shipping ldpi
// hardware, and the clamp keeps the pre-first-size-event state (dpi 0) at a
// usable scale — a deliberate deviation from DisplayMetrics there.
func androidScaleForDPI(dpi float32) float32 {
	if dpi <= 160 {
		return 1
	}
	return dpi / 160
}

func (*device) Locale() bellui.Locale {
	return lang.SystemLocale()
}

func (*device) Orientation() bellui.DeviceOrientation {
	switch currentOrientation {
	case size.OrientationLandscape:
		return bellui.OrientationHorizontalLeft
	default:
		return bellui.OrientationVertical
	}
}

func (*device) IsMobile() bool {
	return true
}

func (*device) IsBrowser() bool {
	return false
}

func (*device) HasKeyboard() bool {
	return false
}

func (d *device) ShowVirtualKeyboard() {
	d.showVirtualKeyboard(mobile.DefaultKeyboard)
}

func (d *device) ShowVirtualKeyboardType(keyboard mobile.KeyboardType) {
	d.showVirtualKeyboard(keyboard)
}

func (d *device) HideVirtualKeyboard() {
	d.hideVirtualKeyboard()
}
