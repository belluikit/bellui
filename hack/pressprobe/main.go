// Command pressprobe shows the press feedback of Button and IconButton (#1085)
// on a device, for a screen recording.
//
// The top row holds a Button of each importance, one with an icon, and two
// IconButtons. Below them, a scrolling list of buttons checks that a touch that
// turns into a scroll leaves no pressed state behind and taps nothing. Every
// tap logs
//
//	pressprobe: TAP <label>
//
// Expected, with the change: a press shows a state layer that spreads from the
// finger while it is down, and finishes and fades after the lift, so a quick
// tap still shows it. A scroll that starts on a list button cancels its press
// and logs no TAP. Before #1085 a tap showed a faint ripple only after the lift.
//
//	go run ./hack/pressprobe
//
// On Android, package it and read logcat for "pressprobe".
package main

import (
	"fmt"
	"log"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

func main() {
	log.SetPrefix("pressprobe: ")
	a := app.NewWithID("com.bellui.pressprobe")
	w := a.NewWindow("pressprobe")

	tap := func(label string) func() {
		return func() { log.Printf("TAP %s", label) }
	}
	button := func(label string, imp widget.Importance) *widget.Button {
		b := widget.NewButton(label, tap(label))
		b.Importance = imp
		return b
	}

	top := container.NewVBox(
		container.NewGridWithColumns(2,
			button("High", widget.HighImportance),
			button("Medium", widget.MediumImportance),
			button("Low", widget.LowImportance),
			button("Danger", widget.DangerImportance),
			widget.NewButtonWithIcon("With icon", theme.ConfirmIcon(), tap("With icon")),
			container.NewHBox(
				widget.NewIconButton(theme.SettingsIcon(), tap("IconButton settings")),
				widget.NewIconButton(theme.DeleteIcon(), tap("IconButton delete")),
			),
		),
		widget.NewSeparator(),
	)

	list := container.NewVBox()
	for i := range 30 {
		label := fmt.Sprintf("List button %d", i)
		list.Add(widget.NewButton(label, tap(label)))
	}

	w.SetContent(container.NewBorder(top, nil, nil, nil, container.NewVScroll(list)))
	w.Resize(geo.NewSize(400, 700))
	bellui.ShowAndRun(w)
}
