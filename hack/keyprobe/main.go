// Command keyprobe live-verifies hardware-key delivery on Android (#793): a
// key acts when it is pressed, a held key repeats, and nothing is doubled or
// dropped in an Entry or in a focusable non-editor.
//
// Every key the target receives is logged to logcat (tag Bellui) as
//
//	keyprobe target=<entry|sink> <what>
//
// so a run is read from the log, not the screen. Tap the Entry or the key
// sink to focus it, then press keys on a hardware keyboard, or let
// keyboard.sh (in this directory) plug in a virtual USB keyboard and press
// them, logging each DOWN and UP with tag KP. `adb emu event send` does not
// work for this: the emulator has no alphabetic keyboard device.
//
// Expected: the sink logs "typed-key"/"typed-rune" right after a key's DOWN,
// never at its UP. A held consonant repeats after about 400 ms. A held vowel
// does not repeat while Gboard shows its accent picker, as in every app. A
// held Escape acts once. The Entry's "text=" grows by one character per
// press, with no doubles.
package main

import (
	"fmt"
	"log"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.NewWithID("io.bellui.keyprobe")
	w := a.NewWindow("keyprobe (#793)")

	entry := widget.NewEntry()
	entry.SetPlaceHolder("Entry: tap, then type")
	entry.OnChanged = func(s string) { log.Printf("keyprobe target=entry text=%q", s) }

	sink := newKeySink()
	w.SetContent(container.NewVBox(
		widget.NewLabel("keyprobe (#793): read logcat, tag Bellui"),
		entry,
		sink,
	))
	log.Print("keyprobe ready")
	bellui.ShowAndRun(w)
}

// keySink is a focusable widget that takes keys but no text, like a chart.
type keySink struct {
	widget.BaseWidget
	label *widget.Label
	n     int
}

func newKeySink() *keySink {
	k := &keySink{label: widget.NewLabel("Key sink: tap here, then press keys")}
	k.ExtendBaseWidget(k)
	return k
}

func (k *keySink) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(widget.NewCard("Key sink", "", k.label))
}

func (k *keySink) Tapped(*event.PointEvent) {
	if c := bellui.CurrentApp().Driver().CanvasForObject(k); c != nil {
		c.Focus(k)
	}
}

func (k *keySink) FocusGained() { log.Print("keyprobe target=sink focus") }
func (k *keySink) FocusLost()   {}

func (k *keySink) TypedRune(r rune) { k.show(fmt.Sprintf("typed-rune %q", r)) }

func (k *keySink) TypedKey(e *event.KeyEvent) { k.show("typed-key " + string(e.Name)) }

func (k *keySink) show(what string) {
	k.n++
	log.Printf("keyprobe target=sink n=%d %s", k.n, what)
	k.label.SetText(fmt.Sprintf("Key sink #%d: %s", k.n, what))
}
