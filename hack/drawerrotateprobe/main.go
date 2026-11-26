// Command drawerrotateprobe live-checks #1467 on a phone: an open
// NavigationDrawer that rotates with the device must stay against the edge
// it opened on and span the new height.
//
// It runs four phases of 8 s, each opening a drawer and logging where it is
// every half second. rotate.sh turns the device 3 s into each phase:
//
//	phase 1: end drawer, opened in portrait, turned to landscape
//	phase 2: end drawer, opened in landscape, turned to portrait
//	phase 3: start drawer, opened in portrait, turned to landscape
//	phase 4: start drawer, opened in landscape, turned to portrait
//	phase 5: start drawer, opened in landscape, turned the other way round
//	         (90° to 270°): the canvas keeps its size, the notch changes side
//
// Each line reads:
//
//	drawerrotateprobe phase=1 side=end canvas=360x780 drawer=x56 y0 304x780 gapEdge=0 gapBottom=0 ok=true
//
// gapEdge is the distance from the drawer to the canvas edge it opened
// against, and gapBottom from its bottom to the canvas bottom. want is the
// width it should have here: 360 plus the system inset on its side, at most
// the canvas less a 56 dp strip of scrim. ok is true while the drawer is
// wholly on the canvas with both gaps 0 and the wanted width. Before the fix,
// phase 1 logs a large gapEdge after the turn (the drawer sits near the other
// side), and phase 2 a drawer past the right edge.
//
// Android, from this directory:
//
//	CGO_LDFLAGS= bellui package --target android/arm64 --app-id com.trendvidia.drawerrotateprobe
//	adb install -r drawerrotateprobe.apk
//	./rotate.sh   # launches the probe, turns the device, restores rotation
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

const phaseLen = 8 * time.Second

func main() {
	a := app.New()
	w := a.NewWindow("drawerrotateprobe")
	w.SetContent(widget.NewLabel("drawerrotateprobe (#1467)"))

	items := []*widget.NavigationDrawerItem{
		{Label: "Home", Icon: theme.HomeIcon()},
		{Label: "Settings", Icon: theme.SettingsIcon()},
	}
	a.Lifecycle().SetOnStarted(func() {
		go run(w, items)
	})
	bellui.ShowAndRun(w)
}

func run(w bellui.Window, items []*widget.NavigationDrawerItem) {
	time.Sleep(2 * time.Second)
	sides := []widget.DrawerSide{widget.DrawerSideEnd, widget.DrawerSideEnd, widget.DrawerSideStart, widget.DrawerSideStart, widget.DrawerSideStart}
	bad := 0
	for i, side := range sides {
		phase := i + 1
		d := widget.NewNavigationDrawer(items, nil)
		d.Side = side
		bellui.DoAndWait(func() { d.Show(w.Canvas()) })
		log.Printf("drawerrotateprobe phase=%d shown", phase)
		end := time.Now().Add(phaseLen)
		lastOK := true
		for time.Now().Before(end) {
			time.Sleep(500 * time.Millisecond)
			var line string
			bellui.DoAndWait(func() { line, lastOK = measure(w.Canvas(), side) })
			log.Printf("drawerrotateprobe phase=%d %s", phase, line)
		}
		if !lastOK {
			bad++
		}
		bellui.DoAndWait(d.Hide)
		log.Printf("drawerrotateprobe phase=%d done", phase)
		time.Sleep(3 * time.Second) // rotate.sh may turn the device here
	}
	if bad == 0 {
		log.Println("drawerrotateprobe PASS: every phase ended with the drawer against its edge")
	} else {
		log.Printf("drawerrotateprobe FAIL: %d of %d phases ended with the drawer misplaced", bad, len(sides))
	}
}

// measure reports where the shown drawer's content is against the canvas.
// It runs on the main goroutine.
func measure(c core.Canvas, side widget.DrawerSide) (string, bool) {
	p, ok := c.Overlays().Top().(*widget.PopUp)
	if !ok {
		return "drawer=none ok=false", false
	}
	cs := c.Size()
	// A pop-up covers the whole canvas from its origin, so its content's
	// position is in canvas coordinates. AbsolutePositionForObject is not:
	// on mobile it measures from the safe area.
	pos := p.Content.Position()
	size := p.Content.Size()
	// The content sits inside the drawer's surface by half the inner padding.
	off := theme.Size(theme.SizeNameInnerPadding) / 2
	surface := geo.NewPos(pos.X-off, pos.Y-off)
	surfaceSize := size.AddWidthHeight(2*off, 2*off)

	atRight := side == widget.DrawerSideEnd // the probe runs left to right
	gapEdge := surface.X
	name := "start"
	if atRight {
		gapEdge = cs.Width - (surface.X + surfaceSize.Width)
		name = "end"
	}
	gapBottom := cs.Height - (surface.Y + surfaceSize.Height)
	areaPos, areaSize := c.InteractiveArea()
	inset := areaPos.X
	if atRight {
		inset = cs.Width - areaPos.X - areaSize.Width
	}
	want := min(360+inset, cs.Width-56)
	onCanvas := surface.X >= -0.5 && surface.X+surfaceSize.Width <= cs.Width+0.5
	good := onCanvas && abs(gapEdge) < 0.5 && abs(gapBottom) < 0.5 && abs(surfaceSize.Width-want) < 0.5
	return fmt.Sprintf("side=%s canvas=%.0fx%.0f drawer=x%.0f y%.0f %.0fx%.0f want=%.0f gapEdge=%.0f gapBottom=%.0f ok=%v",
		name, cs.Width, cs.Height, surface.X, surface.Y, surfaceSize.Width, surfaceSize.Height, want, gapEdge, gapBottom, good), good
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
