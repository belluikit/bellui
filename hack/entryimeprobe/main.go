// Command entryimeprobe is the diagnostic harness for the Android TalkBack
// text-editing bugs (pre-rebrand tracker #572: stuck caret + phantom "space deleted").
//
// It shows a single widget.Entry and logs the bellui-side editing state (text +
// caret row/col) on every change, tagged "BelluiEntry". Paired with the native
// "BelluiIME" InputConnection/TextWatcher trace (the imeTrace logging in
// GoNativeActivity), this captures BOTH sides of what happens per keystroke.
//
// HUMAN device pass (the only way — adb bypasses the soft-keyboard IME):
//  1. Enable TalkBack (Settings ▸ Accessibility ▸ TalkBack).
//  2. adb logcat -c ; adb logcat | grep -E "BelluiIME|BelluiEntry"
//  3. Focus the Entry, type one char (e.g. "a"), then a second ("b"),
//     then Backspace twice. Speak/observe the caret.
//  4. The paired BelluiIME (native calls) + BelluiEntry (bellui text/caret) log is
//     the exact sequence to design the real InputConnection against (pre-rebrand tracker #572).
package main

import (
	"fmt"
	"log"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.NewWithID("io.bellui.entryimeprobe")
	w := a.NewWindow("Entry IME/TalkBack probe (#572)")

	status := widget.NewLabel("text=\"\" caret=(0,0)")
	status.Wrapping = bellui.TextWrapWord

	e := widget.NewEntry()
	e.SetPlaceHolder("Type here with TalkBack on")

	report := func(where string) {
		msg := fmt.Sprintf("BelluiEntry[%s] text=%q caret=row%d col%d", where, e.Text, e.CursorRow, e.CursorColumn)
		log.Print(msg)
		bellui.Do(func() {
			status.SetText(fmt.Sprintf("text=%q caret=(row%d,col%d)", e.Text, e.CursorRow, e.CursorColumn))
		})
	}
	e.OnChanged = func(string) { report("OnChanged") }
	e.OnCursorChanged = func() { report("CursorChanged") }

	w.SetContent(container.NewVBox(
		widget.NewLabelWithStyle("Entry IME / TalkBack diagnostic (#572)", bellui.TextAlignCenter, bellui.TextStyle{Bold: true}),
		widget.NewLabel("logcat -e 'BelluiIME|BelluiEntry' while typing under TalkBack"),
		e,
		widget.NewSeparator(),
		status,
	))
	w.Resize(geo.NewSize(560, 360))
	log.Print("entryimeprobe: ready — enable TalkBack, then type into the Entry")
	bellui.ShowAndRun(w)
}
