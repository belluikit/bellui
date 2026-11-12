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
//
// With -auto (desktop) it presses each segment and each split-button half
// itself, captures the window while the press is held, and compares two
// pixels per part against a capture with nothing pressed: one just inside the
// part's rounded outer corner, which the state layer must leave alone, and one
// just inside its square inner corner, which the layer must change. It logs
// one PASS or FAIL line per part and exits non-zero on any FAIL.
//
//	go run ./hack/partpressprobe -auto
package main

import (
	"flag"
	"image"
	"log"
	"os"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/widget"
)

func main() {
	auto := flag.Bool("auto", false, "press each part, check the captures and exit")
	flag.Parse()
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
	if *auto {
		go func() {
			time.Sleep(2 * time.Second) // let the window show and settle
			failed := check(w, seg, split)
			bellui.Do(a.Quit)
			if failed {
				os.Exit(1)
			}
		}()
	}
	bellui.ShowAndRun(w)
}

// part is one press target: where to press it, and its outer and inner
// corner probe points, all relative to the target.
type part struct {
	name         string
	target       desktop.Mouseable
	obj          core.CanvasObject
	press        geo.Position
	outer, inner geo.Position
}

// check presses each part and reports whether any failed.
func check(w bellui.Window, seg *widget.SegmentedButton, split *widget.SplitButton) (failed bool) {
	var parts []part
	bellui.DoAndWait(func() {
		var segs []core.CanvasObject
		for _, o := range cache.Renderer(seg).Objects() {
			if _, ok := o.(desktop.Mouseable); ok {
				segs = append(segs, o)
			}
		}
		if len(segs) != 3 {
			log.Fatalf("FAIL found %d segments, want 3", len(segs))
		}
		for i, s := range segs {
			sz := s.Size()
			p := part{
				name: []string{"first segment", "middle segment", "last segment"}[i], target: s.(desktop.Mouseable), obj: s,
				press: geo.NewPos(sz.Width/2, sz.Height/2),
			}
			switch i {
			case 0: // rounded left: the left corner is outside, the right inside
				p.outer, p.inner = geo.NewPos(3, 3), geo.NewPos(sz.Width-3, 3)
			case 1: // square: both corners are inside
				p.outer, p.inner = geo.Position{}, geo.NewPos(3, 3)
			case 2:
				p.outer, p.inner = geo.NewPos(sz.Width-3, 3), geo.NewPos(3, 3)
			}
			parts = append(parts, p)
		}
		sz := split.Size()
		drop := sz.Width - 36
		parts = append(
			parts,
			part{"split primary half", split, split, geo.NewPos(drop/2, sz.Height/2), geo.NewPos(3, 3), geo.NewPos(drop-3, 3)},
			part{"split dropdown half", split, split, geo.NewPos(drop+18, sz.Height/2), geo.NewPos(sz.Width-3, 3), geo.NewPos(drop+3, 3)},
		)
	})

	before := capture(w)
	for _, p := range parts {
		bellui.DoAndWait(func() {
			p.target.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary, PointEvent: pointAt(p.press)})
		})
		time.Sleep(500 * time.Millisecond) // the ripple spreads, a frame draws it
		pressed := capture(w)
		bellui.DoAndWait(func() { p.target.MouseUp(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary}) })
		time.Sleep(500 * time.Millisecond)

		ok := true
		if p.outer != (geo.Position{}) && changed(w, before, pressed, p.obj, p.outer) {
			log.Printf("FAIL %s: the state layer spilled into its rounded outer corner", p.name)
			ok = false
		}
		if !changed(w, before, pressed, p.obj, p.inner) {
			log.Printf("FAIL %s: no state layer at its square inner corner", p.name)
			ok = false
		}
		if ok {
			log.Printf("PASS %s", p.name)
		}
		failed = failed || !ok
	}
	return failed
}

func pointAt(pos geo.Position) event.PointEvent { return event.PointEvent{Position: pos} }

// capture returns the window's pixels. Capture runs on the main goroutine,
// and can come back empty while no frame is ready, so it is tried again.
func capture(w bellui.Window) (img image.Image) {
	for range 20 {
		bellui.DoAndWait(func() { img = w.Canvas().Capture() })
		if img != nil {
			return img
		}
		time.Sleep(100 * time.Millisecond)
	}
	log.Fatal("FAIL the window gave no capture")
	return nil
}

// changed reports whether the pixel at pos in obj differs between a and b.
func changed(w bellui.Window, a, b image.Image, obj core.CanvasObject, pos geo.Position) bool {
	var abs geo.Position
	var scale float32
	bellui.DoAndWait(func() {
		abs = bellui.CurrentApp().Driver().AbsolutePositionForObject(obj).Add(pos)
		scale = float32(a.Bounds().Dx()) / w.Canvas().Size().Width
	})
	x, y := int(abs.X*scale), int(abs.Y*scale)
	r1, g1, b1, _ := a.At(x, y).RGBA()
	r2, g2, b2, _ := b.At(x, y).RGBA()
	d := func(p, q uint32) uint32 {
		if p > q {
			return (p - q) >> 8
		}
		return (q - p) >> 8
	}
	return d(r1, r2)+d(g1, g2)+d(b1, b2) >= 6
}
