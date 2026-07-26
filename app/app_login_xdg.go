//go:build !ci && !wasm && !android && !ios && !mobile && (linux || openbsd || freebsd || netbsd) && !tinygo && !noos && !tamago

package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/belluikit/bellui"
)

// autostartFilePath returns the freedesktop autostart .desktop path for this
// app: $XDG_CONFIG_HOME/autostart/<uniqueID>.desktop (usually ~/.config/...).
func (a *belluiApp) autostartFilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	id := a.UniqueID()
	if id == "" {
		return "", fmt.Errorf("cannot manage launch-at-login without an app unique ID")
	}
	return filepath.Join(dir, "autostart", id+".desktop"), nil
}

// SetLaunchAtLogin writes (or removes) a freedesktop autostart entry so
// XDG-compliant desktops start this app at login. Part of the
// [bellui.LaunchAtLoginApp] optional interface.
func (a *belluiApp) SetLaunchAtLogin(enabled bool) error {
	path, err := a.autostartFilePath()
	if err != nil {
		return err
	}

	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	name := a.Metadata().Name
	if name == "" {
		name = a.UniqueID()
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	entry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=%s\nExec=%s\nX-GNOME-Autostart-enabled=true\n",
		name, exe)
	return os.WriteFile(path, []byte(entry), 0o644)
}

// LaunchAtLogin reports whether the autostart entry exists.
func (a *belluiApp) LaunchAtLogin() bool {
	path, err := a.autostartFilePath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

var _ bellui.LaunchAtLoginApp = (*belluiApp)(nil)
