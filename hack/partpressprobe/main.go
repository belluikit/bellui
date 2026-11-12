// Command partpressprobe shows the pressed state of segmented-button segments
// and split-button halves (#1156) on a device, for a screenshot taken while a
// press is held.
//
// A segmented button with three segments sits above a split button. Every
// tap logs
//
//	partpressprobe: TAP <what>
//
// Expected, with the change: a press held on the first segment shows a state
// layer rounded on the left only; on the middle segment, square; on the last,
// rounded on the right only. Nothing spills past the outline and no gap shows
// at a seam. A press on the split button's primary half shows on that half,
// rounded on the left; one on its dropdown half shows there, rounded on the
// right. Before #1156 the segments and the split button showed nothing while
// pressed.
//
//	go run ./hack/partpressprobe
//
// On Android, package it and read logcat for "partpressprobe".
package main

import (
	"log"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func main() {
	log.SetPrefix("partpressprobe: ")
	a := app.NewWithID("com.bellui.partpressprobe")
	w := a.NewWindow("partpressprobe")

	seg := widget.NewSegmentedButton([]string{"Day", "Week", "Month"}, func(i int) {
		log.Printf("TAP segment %d", i)
	})
	split := widget.NewSplitButton("Save", func() { log.Print("TAP split primary") },
		[]*bellui.MenuItem{bellui.NewMenuItem("Save as", func() { log.Print("TAP split menu item") })})

	w.SetContent(container.NewVBox(
		widget.NewLabel("partpressprobe"),
		container.NewCenter(seg),
		container.NewCenter(container.NewGridWrap(geo.NewSize(200, 40), split)),
	))
	w.Resize(geo.NewSize(400, 300))
	bellui.ShowAndRun(w)
}
