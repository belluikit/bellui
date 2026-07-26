// Command prvalidate is the on-device validation harness for the dirty-region
// partial-repaint optimization (#340 — mobile validation gate). It reproduces the
// conditions that leaked on desktop before #334 in a form that is unmistakable on
// a phone screen, and animates continuously so partial repaints fire every frame.
//
// Scene: a tall BRIGHT MAGENTA rectangle inside a vertical Scroll whose viewport
// is shorter than the rectangle, so the rectangle overflows the viewport and only
// the scroll's clip cuts it. Below the scroll sits a status label on a dark band
// that updates every frame (driving a localized dirty region). If the scroll clip
// leaks under partial-repaint culling (the #333 bug, fixed in #334 for the mobile
// mobileTreeWalker), magenta bleeds into the bottom band; a green divider marks
// the viewport edge so any bleed past it is obvious.
//
// Build/run:
//
//	# control (full repaint) and test (partial repaint)
//	bellui package -os android --tags ""              -name prvalidate-full
//	bellui package -os android --tags "partialrepaint" -name prvalidate-partial
//
// CLEAN = magenta stops at the green line; the bottom band shows only "frame N"
// on dark. LEAK = magenta (or stale text) appears below the green line / over the
// label. Compare the two builds.
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
	"github.com/belluikit/bellui/internal/build"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("prvalidate")

	bg := canvas.NewRectangle(color.NRGBA{R: 12, G: 12, B: 16, A: 255})

	// A magenta rectangle far taller than any phone viewport, kept at full size by
	// a no-layout container so the scroll must clip its overflow.
	magenta := canvas.NewRectangle(color.NRGBA{R: 255, G: 0, B: 200, A: 255})
	magenta.SetMinSize(geo.NewSize(2000, 2000))
	magenta.Resize(geo.NewSize(2000, 2000))
	scroll := container.NewVScroll(container.NewWithoutLayout(magenta))

	// Green divider marking the viewport's bottom edge; magenta must never cross it.
	divider := canvas.NewRectangle(color.NRGBA{R: 0, G: 230, B: 80, A: 255})
	divider.SetMinSize(geo.NewSize(10, 4))

	// Status label on a dark band — the per-frame dirty region.
	band := canvas.NewRectangle(color.NRGBA{R: 12, G: 12, B: 16, A: 255})
	status := widget.NewLabel("frame 0")
	bottom := container.NewStack(band, container.NewVBox(divider, status))

	header := widget.NewLabel("partialrepaint=" + fmt.Sprint(build.PartialRepaint))
	content := container.NewBorder(header, bottom, nil, nil, scroll)
	w.SetContent(container.NewStack(bg, content))
	w.Resize(geo.NewSize(420, 720))

	// Animate forever: update the status label every frame so a localized partial
	// repaint fires continuously. Also gently auto-scrolls so the overflow fold
	// sweeps through different offsets.
	go func() {
		time.Sleep(time.Second)
		tick := time.NewTicker(time.Second / 30)
		defer tick.Stop()
		n := 0
		for range tick.C {
			n++
			cur := n
			bellui.Do(func() {
				status.SetText(fmt.Sprintf("frame %d  partial=%v", cur, build.PartialRepaint))
				if cur%4 == 0 {
					off := float32((cur / 4 % 200) * 6)
					scroll.ScrollToOffset(geo.NewPos(0, off))
				}
			})
		}
	}()

	bellui.ShowAndRun(w)
}
