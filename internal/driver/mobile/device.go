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
