//go:build !wasm

package glfw

import (
	"bytes"
	"image/png"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/driver/software"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/internal/svg"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/systray"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"

	"github.com/belluikit/glfw"
)

// Declare conformity with the screens enumeration extension on desktop.
var _ desktop.ScreensDriver = (*gLDriver)(nil)

const systrayIconSize = 64

var (
	systrayIcon     bellui.Resource
	systrayRunning  bool
	systrayTooltip  string
	systrayOnTapped func()
)

// HasSecondaryDisplay returns true if there are more than one screens available.
// Call on the main goroutine.
func (d *gLDriver) HasSecondaryDisplay() bool {
	monitors := glfw.GetMonitors()
	if len(monitors) <= 1 {
		return false
	}

	primaryX, primaryY := monitors[0].GetPos()
	for _, m := range monitors[1:] {
		x, y := m.GetPos()
		if x != primaryX || y != primaryY {
			return true
		}
	}

	return false // all the monitors had same origin, thus mirroring
}

// Screens enumerates the displays attached to the system. Call on the main goroutine.
func (d *gLDriver) Screens() []desktop.ScreenInfo {
	monitors := glfw.GetMonitors()
	primary := glfw.GetPrimaryMonitor()

	screens := make([]desktop.ScreenInfo, 0, len(monitors))
	for _, m := range monitors {
		x, y := m.GetPos()
		mode := m.GetVideoMode()
		if mode == nil {
			continue // display-settings query failed (RDP/virtual display); can't describe this screen
		}
		bounds := desktop.ScreenArea{X: x, Y: y, Width: mode.Width, Height: mode.Height}

		workX, workY, workWidth, workHeight := m.GetWorkarea()
		workArea := desktop.ScreenArea{X: workX, Y: workY, Width: workWidth, Height: workHeight}
		if workWidth == 0 || workHeight == 0 { // not reported on this system
			workArea = bounds
		}

		screens = append(screens, desktop.ScreenInfo{
			Name:     m.GetName(),
			Bounds:   bounds,
			WorkArea: workArea,
			Scale:    getMonitorScale(m),
			Primary:  primary != nil && m.GetName() == primary.GetName(),
		})
	}

	return screens
}

func (d *gLDriver) SetSystemTrayMenu(m *bellui.Menu) {
	if !systrayRunning {
		systrayRunning = true
		d.runSystray(m)
	}

	d.refreshSystray(m)
}

func (d *gLDriver) runSystray(m *bellui.Menu) {
	d.trayStart, d.trayStop = systray.RunWithExternalLoop(func() {
		if systrayIcon != nil {
			d.SetSystemTrayIcon(systrayIcon)
		} else if bellui.CurrentApp().Icon() != nil {
			d.SetSystemTrayIcon(bellui.CurrentApp().Icon())
		} else {
			d.SetSystemTrayIcon(theme.BrokenImageIcon())
		}

		// Some XDG systray crash without a title (See #3678)
		if runtime.GOOS == "linux" || runtime.GOOS == "openbsd" || runtime.GOOS == "freebsd" || runtime.GOOS == "netbsd" {
			app := bellui.CurrentApp()
			title := app.Metadata().Name
			if title == "" {
				title = app.UniqueID()
			}

			systray.SetTitle(title)
		}

		// Apply any tooltip / tap handler set before the tray started.
		if systrayTooltip != "" {
			systray.SetTooltip(systrayTooltip)
		}
		if systrayOnTapped != nil {
			systray.SetOnTapped(systrayOnTapped)
		}

		if m != nil {
			// it must be refreshed after init, so an earlier call would have been ineffective
			runOnMain(func() {
				d.refreshSystray(m)
			})
		}
	}, func() {
		// anything required for tear-down
	})

	// the only way we know the app was asked to quit is if this window is asked to close...
	w := d.CreateWindow("SystrayMonitor")
	w.(*window).create()
	w.SetCloseIntercept(d.Quit)
}

func itemForMenuItem(i *bellui.MenuItem, parent *systray.MenuItem) *systray.MenuItem {
	if i.IsSeparator {
		if parent != nil {
			parent.AddSeparator()
		} else {
			systray.AddSeparator()
		}
		return nil
	}

	var item *systray.MenuItem
	if i.Checked {
		if parent != nil {
			item = parent.AddSubMenuItemCheckbox(i.Label, "", true)
		} else {
			item = systray.AddMenuItemCheckbox(i.Label, "", true)
		}
	} else {
		if parent != nil {
			item = parent.AddSubMenuItem(i.Label, "")
		} else {
			item = systray.AddMenuItem(i.Label, "")
		}
	}
	if i.Disabled {
		item.Disable()
	}
	if i.Icon != nil {
		data := i.Icon.Content()
		if svg.IsResourceSVG(i.Icon) {
			b := &bytes.Buffer{}
			res := i.Icon
			if runtime.GOOS == "windows" && isDark() { // windows menus don't match dark mode so invert icons
				res = theme.NewInvertedThemedResource(i.Icon)
			}
			img := painter.PaintImage(canvas.NewImageFromResource(res), nil, systrayIconSize, systrayIconSize)
			err := png.Encode(b, img)
			if err != nil {
				bellui.LogError("Failed to encode SVG icon for menu", err)
			} else {
				data = b.Bytes()
			}
		}

		img, err := toOSIcon(data)
		if err != nil {
			bellui.LogError("Failed to convert systray icon", err)
		} else {
			if _, ok := i.Icon.(*theme.ThemedResource); ok {
				item.SetTemplateIcon(img, img)
			} else {
				item.SetIcon(img)
			}
		}
	}
	return item
}

func (d *gLDriver) refreshSystray(m *bellui.Menu) {
	d.systrayMenu = m

	// Stop the previous menu's click-listener goroutines before rebuilding, so
	// dynamic tray menus don't leak one goroutine per item on every refresh.
	if d.systrayClickDone != nil {
		close(d.systrayClickDone)
	}
	d.systrayClickDone = make(chan struct{})

	systray.ResetMenu()
	d.refreshSystrayMenu(m, nil, d.systrayClickDone)

	addMissingQuitForMenu(m, d)
}

func (d *gLDriver) refreshSystrayMenu(m *bellui.Menu, parent *systray.MenuItem, done chan struct{}) {
	if m == nil {
		return
	}

	for _, i := range m.Items {
		item := itemForMenuItem(i, parent)
		if item == nil {
			continue // separator
		}
		if i.ChildMenu != nil {
			d.refreshSystrayMenu(i.ChildMenu, item, done)
		}

		fn := i.Action
		go func() {
			for {
				select {
				case <-done:
					return
				case _, ok := <-item.ClickedCh:
					if !ok {
						return
					}
					if fn != nil {
						runOnMain(fn)
					}
				}
			}
		}()
	}
}

func (d *gLDriver) SetSystemTrayIcon(resource bellui.Resource) {
	systrayIcon = resource // in case we need it later

	// only macOS supports SVG system tray
	if runtime.GOOS != "darwin" && svg.IsResourceSVG(resource) {
		img := canvas.NewImageFromResource(resource)
		c := software.NewTransparentCanvas()
		c.SetContent(img)
		c.SetPadded(false)
		c.Resize(geo.NewSquareSize(systrayIconSize))

		buf := &bytes.Buffer{}
		err := png.Encode(buf, c.Capture())
		if err != nil {
			bellui.LogError("Failed to encode SVG icon for system tray icon", err)
			return
		}
		resource = bellui.NewStaticResource(resource.Name()+".png", buf.Bytes())
	}

	img, err := toOSIcon(resource.Content())
	if err != nil {
		bellui.LogError("Failed to convert systray icon", err)
		return
	}

	if _, ok := resource.(*theme.ThemedResource); ok {
		systray.SetTemplateIcon(img, img)
	} else {
		systray.SetIcon(img)
	}
}

func (d *gLDriver) SetSystemTrayWindow(w bellui.Window) {
	if !systrayRunning {
		systrayRunning = true
		d.runSystray(nil)
	}

	w.SetCloseIntercept(w.Hide)
	glw := w.(*window)
	if glw.decorate {
		d.setSystrayOnTapped(glw.Show)
	} else {
		d.setSystrayOnTapped(glw.toggleVisible)
	}
}

// SetSystemTrayTooltip sets the hover tooltip for the tray icon, starting the
// tray if needed and applying the value immediately when it is already running.
func (d *gLDriver) SetSystemTrayTooltip(text string) {
	systrayTooltip = text
	if !systrayRunning {
		systrayRunning = true
		d.runSystray(nil)
		return // onReady applies the stored tooltip once the tray is up
	}
	systray.SetTooltip(text)
}

// SetSystemTrayOnTapped sets a custom primary-tap handler for the tray icon.
// It shares the single underlying tap slot with SetSystemTrayWindow (last set
// wins).
func (d *gLDriver) SetSystemTrayOnTapped(fn func()) {
	var wrapped func()
	if fn != nil {
		// The tray library may invoke the tap callback off the main goroutine;
		// the documented contract delivers the app handler on main.
		wrapped = func() { runOnMain(fn) }
	}
	if !systrayRunning {
		systrayRunning = true
		systrayOnTapped = wrapped
		d.runSystray(nil)
		return // onReady applies the stored handler once the tray is up
	}
	d.setSystrayOnTapped(wrapped)
}

// setSystrayOnTapped is the single point that writes the tray's tap slot, so
// SetSystemTrayWindow and SetSystemTrayOnTapped observe last-set-wins both at
// runtime and via the deferred onReady application.
func (d *gLDriver) setSystrayOnTapped(fn func()) {
	systrayOnTapped = fn
	systray.SetOnTapped(fn)
}

func (d *gLDriver) SystemTrayMenu() *bellui.Menu {
	return d.systrayMenu
}

// this function should be invoked from a goroutine
func (d *gLDriver) catchTerm() {
	terminateSignal := make(chan os.Signal, 1)
	signal.Notify(terminateSignal, syscall.SIGINT, syscall.SIGTERM)

	<-terminateSignal
	bellui.Do(d.Quit)
}

func addMissingQuitForMenu(menu *bellui.Menu, d *gLDriver) {
	localQuit := lang.L("Quit")
	var lastItem *bellui.MenuItem
	if len(menu.Items) > 0 {
		lastItem = menu.Items[len(menu.Items)-1]
		if lastItem.Label == localQuit {
			lastItem.IsQuit = true
		}
	}
	if lastItem == nil || !lastItem.IsQuit { // make sure the menu always has a quit option
		quitItem := bellui.NewMenuItem(localQuit, nil)
		quitItem.IsQuit = true
		menu.Items = append(menu.Items, bellui.NewMenuItemSeparator(), quitItem)
	}
	for _, item := range menu.Items {
		if item.IsQuit && item.Action == nil {
			item.Action = d.Quit
		}
	}
}
