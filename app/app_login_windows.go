//go:build !ci && !android && !ios && !wasm && !tinygo

package app

import (
	"errors"
	"os"

	"golang.org/x/sys/windows/registry"

	"github.com/belluikit/bellui"
)

const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

// runValueName is the registry value name under the Run key for this app.
func (a *belluiApp) runValueName() string {
	name := a.Metadata().Name
	if name == "" {
		name = a.UniqueID()
	}
	return name
}

// SetLaunchAtLogin adds (or removes) an HKCU Run registry value pointing at
// this executable. Part of the [bellui.LaunchAtLoginApp] optional interface.
func (a *belluiApp) SetLaunchAtLogin(enabled bool) error {
	access := uint32(registry.SET_VALUE)
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, access)
	if err != nil {
		return err
	}
	defer key.Close()

	name := a.runValueName()
	if !enabled {
		if err := key.DeleteValue(name); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	// Quote the path so spaces in the install directory are handled.
	return key.SetStringValue(name, `"`+exe+`"`)
}

// LaunchAtLogin reports whether the Run registry value exists.
func (a *belluiApp) LaunchAtLogin() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()

	_, _, err = key.GetStringValue(a.runValueName())
	return err == nil
}

var _ bellui.LaunchAtLoginApp = (*belluiApp)(nil)
