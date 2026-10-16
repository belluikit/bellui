// Command systrayprobe live-verifies the system tray tooltip and custom
// tap-callback extensions (pre-rebrand tracker #511).
//
// It configures a tray icon with a hover tooltip (SetSystemTrayTooltip) and a
// left-click handler (SetSystemTrayOnTapped) that logs each tap, then runs for
// a few seconds. Manual checks: hover the tray icon to see the tooltip; click
// it to see "TRAY TAPPED" logged to stdout.
//
//	go run ./hack/systrayprobe
package main

import (
	"fmt"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/theme"
)

func main() {
	a := app.NewWithID("com.trendvidia.systrayprobe")
	if desk, ok := a.Driver().(desktop.App); ok {
		desk.SetSystemTrayIcon(theme.InfoIcon())
		desk.SetSystemTrayTooltip("Systray probe tooltip (#511)")
		desk.SetSystemTrayOnTapped(func() {
			fmt.Println("TRAY TAPPED at", time.Now().Format("15:04:05"))
		})
		fmt.Println("tray configured: tooltip + onTapped set")
	} else {
		fmt.Println("driver is not desktop.App")
	}

	go func() {
		time.Sleep(6 * time.Second)
		fmt.Println("probe still alive, no panic; quitting")
		bellui.Do(a.Quit)
	}()
	a.Run()
}
