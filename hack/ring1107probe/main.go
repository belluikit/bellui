// Command ring1107probe verifies the iOS VoiceOver per-line ring alignment fix
// (pre-rebrand tracker #1107) on a real iPad. It shows a multi-line widget.Entry pre-filled with
// several short, distinct lines so line-by-line VoiceOver navigation is easy to
// follow and the cursor ring position per line is visually checkable.
//
// Needs a human on the device:
//  1. Enable VoiceOver (Settings > Accessibility, or triple-click side button).
//  2. Double-tap the multi-line Entry to start editing (keyboard rises).
//  3. Rotor -> "Lines"; swipe down to move line by line.
//
// Expected (fixed): the VoiceOver ring surrounds each line, aligned to the
// rendered text — NOT drawn between two lines. Build with -tags accessibility.
//
// Instrumentation: the fix's firstRectForRange: override logs each returned
// rect via os_log (subsystem default, %{public}s) — observe with
// `pymobiledevice3 syslog live` and grep "bellui1107" to confirm the sidecar
// path (not super) is answering VoiceOver.
package main

import (
	"os"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("VoiceOver per-line ring (#1107)")

	multi := widget.NewMultiLineEntry()
	// Distinct, short lines: each is one visual line (no wrap needed), so
	// "navigate by line" maps 1:1 to these and a mis-placed ring is obvious.
	multi.SetText("Line one alpha\nLine two bravo\nLine three charlie\nLine four delta\nLine five echo\nLine six foxtrot")
	multi.Wrapping = bellui.TextWrapOff

	w.SetContent(container.NewVBox(
		widget.NewLabelWithStyle("#1107 — VoiceOver line ring", bellui.TextAlignCenter, bellui.TextStyle{Bold: true}),
		widget.NewLabel("VoiceOver ON → double-tap the field → rotor Lines → swipe down."),
		widget.NewLabel("Ring should hug each line, not sit between two."),
		widget.NewSeparator(),
		multi,
	))
	w.Resize(geo.NewSize(700, 640))

	// Autonomous repro for the bare-view render deadlock (pre-rebrand tracker #1124/#1125): with
	// BELLUI_PROBE_AUTOFOCUS set, programmatically focus the multi-line Entry and
	// select a word a moment after launch — reproducing the tap→focus→
	// keyboard/selection path that wedges bellui's renderer under the bare
	// BelluiTextInputView (BELLUI_IOS_BARE_TEXTINPUT), with NO human interaction, so
	// the diagnose/fix loop can run from `devicectl launch` alone.
	if af := os.Getenv("BELLUI_PROBE_AUTOFOCUS"); af != "" {
		go func() {
			time.Sleep(2 * time.Second)
			bellui.Do(func() {
				switch af {
				case "longpress":
					// Faithful real-tap touch path: focus + word-select + touch
					// selection handles (LongPressed is what the driver dispatches
					// on a mobile hold). The plain Focus/Select modes skip this and
					// so miss the freeze the real tap hits (pre-rebrand tracker #1124).
					multi.LongPressed(&event.PointEvent{Position: geo.NewPos(40, 20)})
				case "select":
					// Word selection + native edit menu, minus the touch handles.
					w.Canvas().Focus(multi)
					multi.Select(0, 9, 0, 14) // "alpha" on line one
				default:
					// "focus": blinking caret only (an unambiguous liveness signal
					// for screenshot-diff checks).
					w.Canvas().Focus(multi)
				}
			})
		}()
	}

	bellui.ShowAndRun(w)
}
