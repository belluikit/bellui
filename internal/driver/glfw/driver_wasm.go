//go:build wasm

package glfw

import (
	"time"

	"github.com/belluikit/bellui"
)

const webDefaultDoubleTapDelay = 300 * time.Millisecond

func (d *gLDriver) SetSystemTrayMenu(m *bellui.Menu) {
	// no-op for wasm apps using this driver
}

func (d *gLDriver) HasSecondaryDisplay() bool {
	return false
}

func (d *gLDriver) catchTerm() {}

func setDisableScreenBlank(disable bool) {
	// awaiting complete support for WakeLock
}

func (d *gLDriver) DoubleTapDelay() time.Duration {
	return webDefaultDoubleTapDelay
}
