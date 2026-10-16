// Command framebench is a frame-time benchmark harness for the dirty-region
// partial-repaint optimization (pre-rebrand issue #9). The GL/WebGPU paint path cannot be
// exercised by the software-painter golden suite, so the optimization can only
// be measured on a real GPU. This harness builds a deliberately heavy, mostly
// static scene and then drives one deterministic update workload at a fixed
// cadence, so the same binary built with and without the partialrepaint tag can
// be compared apples-to-apples.
//
// Measurement is done by the driver's BELLUI_FRAME_DEBUG profiler (internal/driver
// /glfw/frametime.go), which this harness enables. Run one workload per process
// so the profiler's shutdown flush reports the whole run as a single sample set:
//
//	# build both variants
//	go build -o /tmp/framebench.full ./cmd/framebench
//	go build -tags partialrepaint -o /tmp/framebench.partial ./cmd/framebench
//
//	# compare the localized-update case (the one partial repaint should win)
//	/tmp/framebench.full    -workload spot
//	/tmp/framebench.partial -workload spot
//
//	# and the whole-canvas case (partial should gracefully match full)
//	/tmp/framebench.full    -workload full
//	/tmp/framebench.partial -workload full
//
// Workloads:
//
//	spot   — toggle a single small label in one corner each frame. Only a tiny
//	         region changes, so partial repaint should clear+redraw far less.
//	full   — Refresh the whole content tree each frame. The dirty region covers
//	         the canvas, so partial repaint falls back to a full clear; this
//	         workload checks the optimization adds no regression in the worst case.
//	scroll — scroll a long list by one row each frame (a realistic medium case).
package main

import (
	"flag"
	"fmt"
	"image/color"
	"os"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/build"
	"github.com/belluikit/bellui/widget"
)

func main() {
	workload := flag.String("workload", "spot", "update workload: spot | full | scroll")
	frames := flag.Int("frames", 600, "number of update frames to drive before quitting")
	fps := flag.Int("fps", 120, "target refresh cadence (driver paints are capped near 60Hz)")
	rows := flag.Int("rows", 24, "grid rows of static labels (scene CPU-walk weight)")
	cols := flag.Int("cols", 8, "grid columns of static labels")
	overdraw := flag.Int("overdraw", 0, "number of stacked translucent full-canvas gradients (scene GPU-fill weight)")
	fixedWidth := flag.Bool("fixedwidth", false, "spot label keeps a constant width (no sibling relayout on update)")
	flag.Parse()

	// Ensure the driver profiler reports the whole run. A high threshold keeps it
	// from emitting a mid-run partial report; the shutdown flush prints the rest.
	if os.Getenv("BELLUI_FRAME_DEBUG") == "" {
		os.Setenv("BELLUI_FRAME_DEBUG", fmt.Sprintf("%d", *frames+1000))
	}

	a := app.New()
	w := a.NewWindow("framebench")
	w.Resize(geo.NewSize(1200, 820))

	spot, content := buildScene(*workload, *rows, *cols, *overdraw)
	w.SetContent(content)

	fmt.Printf("framebench: workload=%s frames=%d fps=%d rows=%d cols=%d overdraw=%d partialrepaint=%v\n",
		*workload, *frames, *fps, *rows, *cols, *overdraw, build.PartialRepaint)

	go drive(a, *workload, *frames, *fps, *fixedWidth, spot, content)
	bellui.ShowAndRun(w)
}

// buildScene returns a heavy, mostly-static scene plus the small "spot" label the
// spot workload mutates. The static background — a dense grid of styled labels and
// a few cards — makes a full canvas redraw expensive, so the cost the partial path
// avoids is visible against the noise.
func buildScene(workload string, rows, cols, overdraw int) (*widget.Label, core.CanvasObject) {
	bg := canvas.NewRectangle(color.NRGBA{R: 18, G: 18, B: 24, A: 255})

	cells := make([]core.CanvasObject, 0, rows*cols)
	for i := 0; i < rows*cols; i++ {
		l := widget.NewLabel(fmt.Sprintf("item %03d", i))
		cells = append(cells, l)
	}
	grid := container.NewGridWithColumns(cols, cells...)

	// Stacked translucent full-canvas gradients make GPU fill (not the CPU walk)
	// the bottleneck — the case dirty-region culling is meant to win, since the
	// partial path skips these entirely when only a corner changed.
	fillLayers := make([]core.CanvasObject, 0, overdraw)
	for i := 0; i < overdraw; i++ {
		g := canvas.NewLinearGradient(
			color.NRGBA{R: 80, G: 0, B: 120, A: 40},
			color.NRGBA{R: 0, G: 80, B: 120, A: 40}, 0,
		)
		fillLayers = append(fillLayers, g)
	}

	cards := container.NewGridWithColumns(
		3,
		widget.NewCard("Alpha", "subtitle", widget.NewLabel("card body text here")),
		widget.NewCard("Beta", "subtitle", widget.NewLabel("card body text here")),
		widget.NewCard("Gamma", "subtitle", widget.NewLabel("card body text here")),
	)

	spot := widget.NewLabel("spot 0")

	if workload == "scroll" {
		list := widget.NewList(
			func() int { return 5000 },
			func() core.CanvasObject { return widget.NewLabel("row template 0000") },
			func(id widget.ListItemID, o core.CanvasObject) {
				o.(*widget.Label).SetText(fmt.Sprintf("row template %04d", id))
			},
		)
		body := container.NewBorder(cards, spot, nil, nil, list)
		return spot, stack(bg, fillLayers, body)
	}

	body := container.NewBorder(cards, spot, nil, nil,
		container.NewVScroll(grid))
	return spot, stack(bg, fillLayers, body)
}

// stack layers the background, any fill (overdraw) layers, then the body.
func stack(bg core.CanvasObject, fill []core.CanvasObject, body core.CanvasObject) core.CanvasObject {
	objs := make([]core.CanvasObject, 0, len(fill)+2)
	objs = append(objs, bg)
	objs = append(objs, fill...)
	objs = append(objs, body)
	return container.NewStack(objs...)
}

// drive runs the chosen workload for frames-many steps at the requested cadence,
// then quits so the driver flushes its profiler report. It runs on its own
// goroutine and touches widgets only through bellui.DoAndWait.
func drive(a bellui.App, workload string, frames, fps int, fixedWidth bool, spot *widget.Label, content core.CanvasObject) {
	// Let the first frame map and the window settle before measuring.
	time.Sleep(750 * time.Millisecond)

	interval := time.Second / time.Duration(maxInt(fps, 1))
	tick := time.NewTicker(interval)
	defer tick.Stop()

	var scroller *container.Scroll
	if workload == "scroll" {
		scroller = findScroll(content)
	}

	for i := 0; i < frames; i++ {
		<-tick.C
		n := i
		bellui.DoAndWait(func() {
			switch workload {
			case "full":
				content.Refresh()
			case "scroll":
				if scroller != nil {
					scroller.ScrollToOffset(geo.NewPos(0, float32(n*8)))
				}
			default: // spot
				if fixedWidth {
					// Constant-width text: the label's MinSize never changes, so
					// updating it triggers no sibling relayout — isolates a pure
					// dirty-region clear bug from a relayout-move-under-clip bug.
					spot.SetText(fmt.Sprintf("spot %05d", n%100000))
				} else {
					spot.SetText(fmt.Sprintf("spot %d", n))
				}
			}
		})
	}

	// Give the last frames time to paint, then quit (triggers profiler flush).
	time.Sleep(250 * time.Millisecond)
	bellui.Do(func() { a.Quit() })
}

// findScroll returns the first container.Scroll in the tree, or nil.
func findScroll(o core.CanvasObject) *container.Scroll {
	switch t := o.(type) {
	case *container.Scroll:
		return t
	case *bellui.Container:
		for _, c := range t.Objects {
			if s := findScroll(c); s != nil {
				return s
			}
		}
	}
	return nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
