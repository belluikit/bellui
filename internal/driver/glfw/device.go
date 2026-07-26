package glfw

import (
	"runtime"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/lang"
)

type glDevice struct{}

// Declare conformity with Device
var _ bellui.Device = (*glDevice)(nil)

func (*glDevice) Locale() bellui.Locale {
	return lang.SystemLocale()
}

func (*glDevice) Orientation() bellui.DeviceOrientation {
	return bellui.OrientationHorizontalLeft // TODO should we consider the monitor orientation or topmost window?
}

func (*glDevice) HasKeyboard() bool {
	return true // TODO actually check - we could be in tablet mode
}

func (*glDevice) IsBrowser() bool {
	return runtime.GOARCH == "js" || runtime.GOOS == "js"
}
