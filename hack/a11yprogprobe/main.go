// Command a11yprogprobe verifies #763: the Android IME state-mirror (the hidden
// shadow EditText that TalkBack reads) must track a *programmatic* Entry text
// change while the Entry holds focus — SetText / binding write / undo /
// autocomplete — not just keystrokes.
//
// It keeps a single Entry focused and, on a repeating timer, programmatically
// rewrites its text WITHOUT moving focus (so the shadow EditText stays visible
// and the keyboard stays up). The per-frame syncKeyboardState() in the mobile
// driver (#771) should push each new value into the shadow within one paint
// frame.
//
// adb verification (no human TalkBack needed — the mechanism is TalkBack-
// independent; TalkBack merely reads the shadow this proves is fresh):
//  1. Install + launch; the Entry auto-focuses and the soft keyboard rises,
//     making the shadow EditText visible in the view tree.
//  2. Watch the value climb: `adb exec-out uiautomator dump /dev/tty` (or dump
//     to a file and pull) — the EditText node's text= must equal the label's
//     "prog-N" value, updating every ~2s with no input.
//  3. Confirm text= is never stale relative to the on-screen label.
package main

import (
	"fmt"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.NewWithID("io.bellui.a11yprogprobe")
	w := a.NewWindow("Programmatic mirror probe (#763)")

	e := widget.NewEntry()
	e.SetText("prog-0")

	status := widget.NewLabel("shadow should read: prog-0")
	status.Wrapping = bellui.TextWrapWord

	w.SetContent(container.NewVBox(
		widget.NewLabelWithStyle("#763 programmatic mirror", bellui.TextAlignCenter, bellui.TextStyle{Bold: true}),
		widget.NewLabel("Entry is auto-focused; text is rewritten every 2s\nwith NO keystroke. uiautomator dump the EditText."),
		e,
		widget.NewSeparator(),
		status,
	))
	w.Resize(geo.NewSize(560, 360))

	// Focus the Entry once shown (raises the keyboard + shadow EditText) and
	// drive the programmatic changes without ever moving focus.
	go func() {
		time.Sleep(1500 * time.Millisecond)
		bellui.Do(func() { w.Canvas().Focus(e) })

		n := 0
		for range time.Tick(2 * time.Second) {
			n++
			val := fmt.Sprintf("prog-%d", n)
			bellui.Do(func() {
				e.SetText(val) // programmatic: no keystroke, focus unchanged
				status.SetText("shadow should read: " + val)
			})
		}
	}()

	bellui.ShowAndRun(w)
}
