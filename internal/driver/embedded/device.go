package embedded

import (
	"github.com/belluikit/bellui"
)

type noosDevice struct{}

func (n *noosDevice) Orientation() bellui.DeviceOrientation {
	return bellui.OrientationVertical
}

func (n *noosDevice) IsMobile() bool {
	return false
}

func (n *noosDevice) IsBrowser() bool {
	return false
}

func (n *noosDevice) HasKeyboard() bool {
	return true
}

func (n *noosDevice) SystemScaleForWindow(bellui.Window) float32 {
	return 1.0
}

func (n *noosDevice) Locale() bellui.Locale {
	return "en"
}
