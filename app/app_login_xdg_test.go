//go:build !ci && !wasm && !android && !ios && !mobile && (linux || openbsd || freebsd || netbsd) && !tinygo && !noos && !tamago

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLaunchAtLogin_XDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	a := &belluiApp{uniqueID: "io.bellui.test.login"}
	entry := filepath.Join(dir, "autostart", "io.bellui.test.login.desktop")

	// Initially not registered.
	assert.False(t, a.LaunchAtLogin())

	// Enable: file is written and reported.
	require.NoError(t, a.SetLaunchAtLogin(true))
	assert.True(t, a.LaunchAtLogin())
	data, err := os.ReadFile(entry)
	require.NoError(t, err)
	assert.Contains(t, string(data), "[Desktop Entry]")
	assert.Contains(t, string(data), "Exec=")

	// Disable: file removed and reported.
	require.NoError(t, a.SetLaunchAtLogin(false))
	assert.False(t, a.LaunchAtLogin())
	_, err = os.Stat(entry)
	assert.True(t, os.IsNotExist(err))

	// Disabling again is idempotent (no error when already absent).
	assert.NoError(t, a.SetLaunchAtLogin(false))
}
