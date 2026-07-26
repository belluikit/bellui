// Command echoprobe966 is the on-device verification harness for #966
// (shadow-EditText echo bugs): hardware-key double insertion, and the
// password mask leaking into the next field on focus switch.
//
// Layout: Entry A → password Entry P → Entry B, in Tab traversal order.
// Entry A is focused programmatically at startup, so the whole pass is
// drivable from adb hardware key events alone (no taps — TalkBack may be on):
//
//	adb logcat -c ; adb logcat -s Bellui966 &
//	adb shell input text 'abc'          # into A: expect text="abc", no doubling
//	adb shell input keyevent 61         # Tab → P
//	adb shell input text 'secret'       # into P
//	adb shell input keyevent 61         # Tab → B
//	adb shell input text 'xy'           # into B: expect exactly "xy" —
//	                                    # no mask (•) chars, no "secret" leak
//
// Every OnChanged logs "Bellui966 [X] text=..." so the full text of all three
// fields is auditable from logcat after the pass.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.NewWithID("io.bellui.echoprobe966")
	w := a.NewWindow("#966 shadow-echo probe")

	status := widget.NewLabel("")
	status.Wrapping = bellui.TextWrapWord

	entryA := widget.NewEntry()
	entryA.SetPlaceHolder("A: plain")
	entryP := widget.NewPasswordEntry()
	entryP.SetPlaceHolder("P: password")
	entryB := widget.NewEntry()
	entryB.SetPlaceHolder("B: plain")

	report := func(which string, e *widget.Entry) func(string) {
		return func(string) {
			msg := fmt.Sprintf("[%s] text=%q caret=(row%d,col%d)", which, e.Text, e.CursorRow, e.CursorColumn)
			log.Print("Bellui966 ", msg)
			bellui.Do(func() {
				status.SetText(fmt.Sprintf("A=%q P=%q B=%q", entryA.Text, entryP.Text, entryB.Text))
			})
		}
	}
	entryA.OnChanged = report("A", entryA)
	entryP.OnChanged = report("P", entryP)
	entryB.OnChanged = report("B", entryB)

	w.SetContent(container.NewVBox(
		widget.NewLabelWithStyle("#966: hw-key double insertion + mask leak", bellui.TextAlignCenter, bellui.TextStyle{Bold: true}),
		entryA,
		entryP,
		entryB,
		widget.NewSeparator(),
		status,
	))
	w.Resize(geo.NewSize(560, 420))

	go func() {
		time.Sleep(2 * time.Second)
		bellui.Do(func() {
			w.Canvas().Focus(entryA)
			log.Print("Bellui966 focused entry A")
		})
	}()

	log.Print("Bellui966 ready")
	bellui.ShowAndRun(w)
}
