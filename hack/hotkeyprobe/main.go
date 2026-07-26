// Command hotkeyprobe live-verifies global (system-wide) hotkey registration
// (#518).
//
// It registers Cmd+Shift+9 (Ctrl+Shift+9 on non-macOS) as a global shortcut
// that logs when it fires — even while the probe window is unfocused. Press the
// combination from any application to confirm. On macOS the probe also attempts
// to synthesize the keypress via System Events (needs Accessibility permission
// for the launching terminal); if that is denied, registration still succeeds
// and a real keypress will fire the handler.
//
//	go run ./hack/hotkeyprobe
package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("Global Hotkey Probe (#518)")
	w.SetContent(widget.NewLabel("Press Cmd/Ctrl+Shift+9 (even unfocused)"))

	gs, ok := a.Driver().(desktop.GlobalShortcutApp)
	if !ok {
		fmt.Println("driver does not support global shortcuts")
		return
	}

	mod := event.KeyModifierSuper
	if runtime.GOOS != "darwin" {
		mod = event.KeyModifierControl
	}
	sc := &desktop.CustomShortcut{KeyName: event.Key9, Modifier: mod | event.KeyModifierShift}

	err := gs.RegisterGlobalShortcut(sc, func() {
		fmt.Println(">>> global hotkey fired at", time.Now().Format("15:04:05"))
	})
	fmt.Println("RegisterGlobalShortcut err =", err)

	if runtime.GOOS == "darwin" && err == nil {
		go func() {
			time.Sleep(1500 * time.Millisecond)
			_ = exec.Command("osascript", "-e",
				`tell application "System Events" to key code 25 using {command down, shift down}`).Run()
		}()
	}

	w.Show()
	a.Run()
}
