//go:build !ci && !wasm && !android && !ios && !mobile && !tinygo && !noos && !tamago && (darwin || windows || linux || openbsd || freebsd || netbsd)

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/test"
)

// trayDriver is a test driver that records which tray method was called with
// which value, so each app forwarder is checked against its own driver method.
type trayDriver struct {
	bellui.Driver
	calls []string
	got   []any
}

func (d *trayDriver) record(name string, v any) {
	d.calls = append(d.calls, name)
	d.got = append(d.got, v)
}

func (d *trayDriver) SetSystemTrayMenu(m *bellui.Menu)    { d.record("Menu", m) }
func (d *trayDriver) SetSystemTrayIcon(r bellui.Resource) { d.record("Icon", r) }
func (d *trayDriver) SetSystemTrayWindow(w bellui.Window) { d.record("Window", w) }
func (d *trayDriver) SetSystemTrayTooltip(s string)       { d.record("Tooltip", s) }
func (d *trayDriver) SetSystemTrayOnTapped(fn func())     { d.record("OnTapped", fn) }

func TestBelluiApp_SystemTrayForwardsToDriver(t *testing.T) {
	drv := &trayDriver{Driver: test.NewDriver()}
	a := newAppWithDriver(drv, test.NewClipboard(), "io.bellui.test")

	desk, ok := a.(desktop.App)
	if !assert.True(t, ok, "the app must satisfy desktop.App, not only the driver") {
		return
	}

	menu := bellui.NewMenu("tray")
	icon := &bellui.StaticResource{StaticName: "tray.png"}
	win := drv.CreateWindow("tray")
	tapped := 0
	desk.SetSystemTrayMenu(menu)
	desk.SetSystemTrayIcon(icon)
	desk.SetSystemTrayWindow(win)
	desk.SetSystemTrayTooltip("hover")
	desk.SetSystemTrayOnTapped(func() { tapped++ })

	assert.Equal(t, []string{"Menu", "Icon", "Window", "Tooltip", "OnTapped"}, drv.calls)
	if !assert.Len(t, drv.got, 5) {
		return
	}
	assert.Same(t, menu, drv.got[0])
	assert.Same(t, icon, drv.got[1])
	assert.Equal(t, win, drv.got[2])
	assert.Equal(t, "hover", drv.got[3])
	drv.got[4].(func())()
	assert.Equal(t, 1, tapped, "the tap handler reaching the driver must be the one passed in")
}

// A driver without tray support (anything but glfw) makes every tray method a
// no-op. Now that the desktop.App assertion succeeds on the app, these are
// reachable through the documented check, so none of them may panic.
func TestBelluiApp_SystemTrayWithoutTrayDriver(t *testing.T) {
	drv := test.NewDriver()
	_, isTray := drv.(desktop.App)
	if !assert.False(t, isTray, "fixture: the test driver must not support a tray") {
		return
	}
	a := newAppWithDriver(drv, test.NewClipboard(), "io.bellui.test")

	desk := a.(desktop.App)
	assert.NotPanics(t, func() { desk.SetSystemTrayMenu(bellui.NewMenu("tray")) })
	assert.NotPanics(t, func() { desk.SetSystemTrayIcon(&bellui.StaticResource{StaticName: "tray.png"}) })
	assert.NotPanics(t, func() { desk.SetSystemTrayWindow(drv.CreateWindow("tray")) })
	assert.NotPanics(t, func() { desk.SetSystemTrayTooltip("hover") })
	assert.NotPanics(t, func() { desk.SetSystemTrayOnTapped(nil) })
}
