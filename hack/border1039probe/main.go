// Live-verify probe for #1039 (EnsureMinSize dropped-update fix).
//
// Two scenarios on the real glfw driver:
//  1. Re-attach: a Border with a min-size side scroll is swapped out of and
//     back into a Split slot (the goed App Map toggle shape); the side pane
//     must keep its width.
//  2. Same-frame sibling min-size changes: an early sibling's min size and a
//     min-size change absorbed inside a later sibling's Scroll happen in one
//     bellui.Do batch. Before the fix, the single pending-parent pointer in
//     EnsureMinSize dropped the first sibling's parent re-layout, freezing
//     the stale size permanently.
//
// Run: go run ./hack/border1039probe — prints PASS/FAIL lines and quits.
package main

import (
	"fmt"
	"image/color"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("border 1039 probe")

	// Scenario 1: Border side pane across re-attachment.
	toolbar := container.NewHBox(widget.NewButton("Add Note", nil), widget.NewButton("Undo", nil))
	status := widget.NewLabel("status")
	center := canvas.NewRectangle(color.NRGBA{R: 0x30, G: 0x60, B: 0x90, A: 0xff})
	props := widget.NewLabel("properties\npanel\ncontent")
	side := container.NewVScroll(props)
	side.SetMinSize(geo.NewSize(240, 0))
	border := container.NewBorder(toolbar, status, nil, side, center)
	editor := widget.NewLabelWithStyle("EDITOR AREA", bellui.TextAlignCenter, bellui.TextStyle{})
	split := container.NewHSplit(border, widget.NewLabel("trailing pane"))
	split.Offset = 0.8

	// Scenario 2: same-frame sibling min-size changes under a slack parent.
	first := canvas.NewRectangle(color.NRGBA{G: 0xa0, A: 0xff})
	first.SetMinSize(geo.NewSize(100, 50))
	inner := canvas.NewRectangle(color.NRGBA{R: 0xa0, A: 0xff})
	inner.SetMinSize(geo.NewSize(50, 50))
	absorber := container.NewScroll(inner) // both-axes scroll: constant min size
	row := container.NewHBox(first, absorber)

	w.SetContent(container.NewVSplit(split, container.NewStack(row)))
	w.Resize(geo.NewSize(1000, 700))

	check := func(tag string, got, want float32) {
		verdict := "PASS"
		if got < want {
			verdict = "FAIL"
		}
		fmt.Printf("[%s] %s: got %v, want >= %v\n", tag, verdict, got, want)
	}

	go func() {
		step := func(f func()) { time.Sleep(700 * time.Millisecond); bellui.Do(f) }

		step(func() { check("first attach", side.Size().Width, 240) })
		step(func() { split.Leading = editor; split.Refresh() })
		step(func() { split.Leading = border; split.Refresh() })
		step(func() { check("re-attach", side.Size().Width, 240) })

		// Same bellui.Do batch: early sibling grows + absorbed inner change.
		step(func() {
			first.SetMinSize(geo.NewSize(120, 60))
			inner.SetMinSize(geo.NewSize(60, 60))
			first.Refresh()
			inner.Refresh()
		})
		step(func() { check("same-frame siblings", first.Size().Width, 120) })
		step(a.Quit)
	}()

	bellui.ShowAndRun(w)
}
