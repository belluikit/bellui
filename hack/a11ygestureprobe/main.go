// Command a11ygestureprobe verifies pre-rebrand tracker #762: TalkBack accessibility gestures —
// scroll, long-press and slider adjust — must route to the corresponding Bellui
// behaviours even though Bellui consumes raw touch through the native input
// queue (which bypasses the framework's touch-exploration filter, so the pre-rebrand tracker #761
// tap-gate suppresses raw gestures under TalkBack).
//
// It shows:
//   - a scrollable List of 60 rows (ACTION_SCROLL_FORWARD/BACKWARD → the List's
//     scroller),
//   - a custom long-pressable button (ACTION_LONG_CLICK → LongPressed), and
//   - a Slider (ACTION_SCROLL_FORWARD/BACKWARD → increment/decrement, the
//     pre-existing adjustable path).
//
// A status label reports the last routed gesture so the effect is visible in an
// `adb exec-out screencap` even when the TalkBack audio is not capturable.
//
// adb verification (uiautomator can perform node actions directly):
//  1. Enable TalkBack (or rely on the uiautomator dump registering as an a11y
//     client so the overlay is published).
//  2. `adb exec-out uiautomator dump` → confirm the row nodes report
//     scrollable=true / actions incl. ACTION_SCROLL_FORWARD, and the long-press
//     node reports long-clickable=true.
//  3. Perform the actions and screencap the status label / list position.
package main

import (
	"fmt"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// longPressButton is a minimal accessible, long-pressable widget.
type longPressButton struct {
	widget.BaseWidget
	label   *widget.Label
	onLong  func()
	onTap   func()
	caption string
}

func newLongPressButton(caption string, onTap, onLong func()) *longPressButton {
	b := &longPressButton{caption: caption, onTap: onTap, onLong: onLong}
	b.label = widget.NewLabel(caption)
	b.ExtendBaseWidget(b)
	return b
}

func (b *longPressButton) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewPadded(b.label))
}

func (b *longPressButton) Tapped(*event.PointEvent)      { b.onTap() }
func (b *longPressButton) LongPressed(*event.PointEvent) { b.onLong() }

func (b *longPressButton) AccessibleRole() core.AccessibleRole { return core.RoleButton }
func (b *longPressButton) AccessibleLabel() string             { return b.caption }

func main() {
	a := app.NewWithID("io.bellui.a11ygestureprobe")
	w := a.NewWindow("A11y gesture probe (#762)")

	status := widget.NewLabel("last gesture: (none)")
	status.Wrapping = bellui.TextWrapWord
	set := func(s string) { bellui.Do(func() { status.SetText("last gesture: " + s) }) }

	// Scrollable list.
	data := make([]string, 60)
	for i := range data {
		data[i] = fmt.Sprintf("Row %d", i)
	}
	list := widget.NewList(
		func() int { return len(data) },
		func() core.CanvasObject { return widget.NewLabel("template") },
		func(id widget.ListItemID, o core.CanvasObject) { o.(*widget.Label).SetText(data[id]) },
	)

	// Long-pressable button.
	lpb := newLongPressButton(
		"Long-press me",
		func() { set("tap on long-press button") },
		func() { set("LONG PRESS routed!") },
	)

	// Slider (adjustable — the pre-existing increment/decrement path).
	slider := widget.NewSlider(0, 100)
	slider.OnChanged = func(v float64) { set(fmt.Sprintf("slider = %.0f", v)) }

	top := container.NewVBox(
		widget.NewLabelWithStyle("#762 a11y gestures", bellui.TextAlignCenter, bellui.TextStyle{Bold: true}),
		lpb,
		slider,
		status,
		widget.NewSeparator(),
	)
	w.SetContent(container.NewBorder(top, nil, nil, nil, list))
	w.Resize(geo.NewSize(560, 720))
	bellui.ShowAndRun(w)
}
