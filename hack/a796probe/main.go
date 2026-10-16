// Command a796probe verifies pre-rebrand tracker #796 (RangeSlider + TimePicker-dial screen-reader
// adjust) and the mobile a11y proxy-View reconcile.
//
// Adjustable widgets (Slider reference, RangeSlider, TimePicker hour/minute)
// exercise the TalkBack adjust gesture (RangeInfo + SeekBar class + a real value
// via stateDescription, re-announced on change). The List and the dialog
// exercise the reconcile's structural paths — proxy Views added / removed /
// reordered as rows realize/derealize and as the dialog subtree appears and
// disappears — where a full teardown/rebuild used to drop TalkBack's focus.
//
// A status label echoes the last change so the effect is visible in an
// `adb exec-out screencap` even without capturable TalkBack audio.
package main

import (
	"fmt"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/dialog"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.NewWithID("io.bellui.a796probe")
	w := a.NewWindow("A11y adjust probe (#796)")

	status := widget.NewLabel("last change: (none)")
	status.Wrapping = bellui.TextWrapWord
	set := func(s string) { bellui.Do(func() { status.SetText("last change: " + s) }) }

	// Reference: a plain Slider (already adjustable).
	slider := widget.NewSlider(0, 100)
	slider.Step = 5
	slider.SetValue(50)
	slider.OnChanged = func(v float64) { set(fmt.Sprintf("slider = %.0f", v)) }

	// pre-rebrand tracker #796 subject: a two-thumb RangeSlider.
	rs := widget.NewRangeSlider(0, 100)
	rs.Step = 5
	rs.SetValues(20, 80)
	rs.OnChanged = func(lo, hi float64) { set(fmt.Sprintf("range = %.0f..%.0f", lo, hi)) }

	// pre-rebrand tracker #796 subject: a TimePicker (hour/minute header fields are adjustable).
	tp := widget.NewTimePicker(10, 30, true, func(h, m int) { set(fmt.Sprintf("time = %02d:%02d", h, m)) })

	// Reconcile subject: a dialog adds an accessible subtree, then removes it on
	// close — focus must return cleanly to where it was.
	dlgBtn := widget.NewButton("Open dialog", func() {
		dialog.ShowInformation("Dialog",
			"This dialog adds accessibility nodes.\nClose it and focus should return cleanly.", w)
	})

	// Reconcile subject: a scrollable list whose rows realize/derealize on
	// scroll, exercising proxy-View add/remove/reorder.
	rows := make([]string, 40)
	for i := range rows {
		rows[i] = fmt.Sprintf("List row %d", i)
	}
	list := widget.NewList(
		func() int { return len(rows) },
		func() core.CanvasObject { return widget.NewLabel("template") },
		func(id widget.ListItemID, o core.CanvasObject) { o.(*widget.Label).SetText(rows[id]) },
	)

	top := container.NewVBox(
		widget.NewLabelWithStyle("#796 adjust + reconcile", bellui.TextAlignCenter, bellui.TextStyle{Bold: true}),
		dlgBtn,
		widget.NewLabel("Slider (reference):"),
		slider,
		widget.NewLabel("RangeSlider (swipe=adjust active thumb):"),
		rs,
		status,
		widget.NewLabel("TimePicker (Hour/Minute fields adjust):"),
		tp,
		widget.NewSeparator(),
		widget.NewLabel("List (scroll — rows realize/derealize):"),
	)
	// Border gives the List a bounded height so it actually scrolls.
	w.SetContent(container.NewBorder(top, nil, nil, nil, list))
	w.Resize(geo.NewSize(560, 900))
	bellui.ShowAndRun(w)
}
