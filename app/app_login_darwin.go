//go:build !ci && !ios && !wasm && !mobile && !tinygo

package app

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework ServiceManagement

int setLaunchAtLogin(int enable);
int launchAtLoginEnabled(void);
int launchAtLoginSupported(void);
*/
import "C"

import (
	"errors"

	"github.com/belluikit/bellui"
)

// SetLaunchAtLogin registers or unregisters this app as a login item via
// SMAppService (macOS 13+). It requires a bundled app; an unbundled binary
// returns an error. Part of the [bellui.LaunchAtLoginApp] optional interface.
func (a *belluiApp) SetLaunchAtLogin(enabled bool) error {
	if C.launchAtLoginSupported() == 0 {
		return errors.New("launch at login requires macOS 13+ and a bundled app")
	}

	en := C.int(0)
	if enabled {
		en = 1
	}
	if C.setLaunchAtLogin(en) != 0 {
		return errors.New("failed to update launch-at-login registration")
	}
	return nil
}

// LaunchAtLogin reports whether this app is registered as an enabled login item.
func (a *belluiApp) LaunchAtLogin() bool {
	return C.launchAtLoginEnabled() != 0
}

var _ bellui.LaunchAtLoginApp = (*belluiApp)(nil)
