package test

import (
	"runtime"

	"github.com/belluikit/bellui"
)

type device struct{}

// Declare conformity with Device
var _ bellui.Device = (*device)(nil)

func (d *device) Orientation() bellui.DeviceOrientation {
	return bellui.OrientationVertical
}

func (d *device) HasKeyboard() bool {
	return false
}

func (d *device) SystemScale() float32 {
	return d.SystemScaleForWindow(nil)
}

func (d *device) SystemScaleForWindow(bellui.Window) float32 {
	return 1
}

func (d *device) Locale() bellui.Locale {
	return "en"
}

func (*device) IsBrowser() bool {
	return runtime.GOARCH == "js" || runtime.GOOS == "js"
}
