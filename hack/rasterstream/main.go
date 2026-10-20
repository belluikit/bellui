// Command rasterstream is the live-verify probe for canvas.Raster streaming
// (#34) — the path a video player drives with SetPixels.
//
//	go run ./hack/rasterstream          # default painter (WebGPU on desktop)
//	go run -tags gl ./hack/rasterstream # GL painter
//
// The left raster has NO Generator at all, exactly as the SetPixels
// documentation allows: under the old WebGPU painter it segfaulted on the first
// paint. The right one has a Generator as well, the workaround fyplayer adopted:
// it used to freeze on frame 1 because the texture was cached forever.
//
// PASS: both squares animate a moving colour band and the frame counter climbs.
// FAIL: a crash on startup (issue 1), or a still image while the counter climbs
// (issue 2). Resize the window to confirm the texture is reallocated cleanly.
//
// The third square carries the same frames at Translucency 0.5, which the
// WebGPU and software painters used to ignore entirely (#38).
//
// -auto does those checks without a human: it captures the live canvas twice a
// few frames apart and reports, per raster, how many pixels changed inside that
// raster's own bounds (a frozen texture scores 0), then compares the opaque and
// translucent squares — identical colour means Translucency was dropped.
//
//	go run ./hack/rasterstream -auto
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

const (
	frameW = 160
	frameH = 160
)

// renderFrame paints a colour band whose position tracks n, so a frozen texture
// is immediately obvious.
func renderFrame(n int, tint color.RGBA) []byte {
	pix := make([]byte, frameW*frameH*4)
	band := (n * 3) % frameH
	for y := 0; y < frameH; y++ {
		hot := y >= band && y < band+24
		for x := 0; x < frameW; x++ {
			i := (y*frameW + x) * 4
			if hot {
				pix[i], pix[i+1], pix[i+2], pix[i+3] = 255, 255, 255, 255
			} else {
				pix[i], pix[i+1], pix[i+2], pix[i+3] = tint.R, tint.G, tint.B, 255
			}
		}
	}
	return pix
}

// objectRect maps an object's canvas position/size to pixel bounds in a capture.
func objectRect(c core.Canvas, obj core.CanvasObject, img image.Image) image.Rectangle {
	pos := bellui.CurrentApp().Driver().AbsolutePositionForObject(obj)
	size := obj.Size()
	// The capture is in framebuffer pixels, which Scale alone does not give
	// on a high-density output (#556).
	s := float32(img.Bounds().Dx()) / c.Size().Width
	return image.Rect(
		int(pos.X*s), int(pos.Y*s),
		int((pos.X+size.Width)*s), int((pos.Y+size.Height)*s),
	)
}

// changedPixels counts pixels differing between two captures inside r.
func changedPixels(a, b image.Image, r image.Rectangle) int {
	r = r.Intersect(a.Bounds()).Intersect(b.Bounds())
	changed := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				changed++
			}
		}
	}
	return changed
}

// runAuto captures the canvas twice and reports whether each raster's own area
// actually changed between them. Runs off the main goroutine; every canvas
// touch hops back on via bellui.Do.
func runAuto(a bellui.App, w bellui.Window, rasters map[string]core.CanvasObject, opaque, faded core.CanvasObject) {
	capture := func() image.Image {
		var img image.Image
		bellui.DoAndWait(func() { img = w.Canvas().Capture() })
		return img
	}

	time.Sleep(2 * time.Second) // let the window map and a few frames land
	first := capture()
	time.Sleep(time.Second)
	second := capture()

	if first == nil || second == nil {
		fmt.Println("FAIL: canvas capture unavailable")
		bellui.Do(func() { a.Quit() })
		os.Exit(1)
	}

	failed := false
	for name, obj := range rasters {
		var r image.Rectangle
		bellui.DoAndWait(func() { r = objectRect(w.Canvas(), obj, first) })
		n := changedPixels(first, second, r)
		verdict := "PASS"
		if n == 0 {
			verdict, failed = "FAIL (frozen)", true
		}
		fmt.Printf("%-24s %s — %d pixels changed in %v\n", name, verdict, n, r)
	}

	// Translucency: the same frame drawn opaque and at 0.5 must not land on the
	// canvas as the same colour.
	var opaqueRect, fadedRect image.Rectangle
	bellui.DoAndWait(func() {
		opaqueRect = objectRect(w.Canvas(), opaque, second)
		fadedRect = objectRect(w.Canvas(), faded, second)
	})
	oc := second.At(centreOf(opaqueRect))
	fc := second.At(centreOf(fadedRect))
	if sameColor(oc, fc) {
		fmt.Printf("%-24s FAIL — Translucency ignored, both %v\n", "Translucency 0.5", oc)
		failed = true
	} else {
		fmt.Printf("%-24s PASS — opaque %v vs translucent %v\n", "Translucency 0.5", oc, fc)
	}

	bellui.Do(func() { a.Quit() })
	if failed {
		os.Exit(1)
	}
}

func centreOf(r image.Rectangle) (x, y int) {
	return (r.Min.X + r.Max.X) / 2, (r.Min.Y + r.Max.Y) / 2
}

func sameColor(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

func main() {
	auto := flag.Bool("auto", false, "capture two frames, verify both rasters animate, then exit")
	flag.Parse()

	a := app.New()
	w := a.NewWindow("Raster streaming probe (#34)")

	// Issue 1: streaming raster with no Generator — this is the documented
	// SetPixels-only usage that used to crash under WebGPU.
	noGen := canvas.NewRaster(nil)
	noGen.Streaming = true
	noGen.SetMinSize(geo.NewSize(frameW, frameH))

	// Issue 2: the fyplayer workaround — a Generator is supplied purely to dodge
	// the crash, so a stale texture shows as a frozen image rather than a panic.
	withGen := canvas.NewRaster(func(w, h int) image.Image {
		return image.NewRGBA(image.Rect(0, 0, w, h))
	})
	withGen.Streaming = true
	withGen.SetMinSize(geo.NewSize(frameW, frameH))

	// #38: the same frames, faded. GL always honoured Translucency; WebGPU and
	// the software painter passed a hard-coded opaque alpha.
	faded := canvas.NewRaster(nil)
	faded.Streaming = true
	faded.Translucency = 0.5
	faded.SetMinSize(geo.NewSize(frameW, frameH))

	status := widget.NewLabel("frame 0")

	frame := 0
	anim := bellui.NewAnimation(40*time.Millisecond, func(float32) {
		frame++
		noGen.SetPixels(renderFrame(frame, color.RGBA{R: 30, G: 60, B: 160}), frameW, frameH)
		withGen.SetPixels(renderFrame(frame, color.RGBA{R: 160, G: 60, B: 30}), frameW, frameH)
		faded.SetPixels(renderFrame(frame, color.RGBA{R: 30, G: 60, B: 160}), frameW, frameH)
		status.SetText(fmt.Sprintf("frame %d — both squares must animate", frame))
	})
	anim.RepeatCount = bellui.AnimationRepeatForever

	w.SetContent(container.NewBorder(
		widget.NewLabel("SetPixels only  |  SetPixels + Generator  |  SetPixels, Translucency 0.5"),
		status, nil, nil,
		container.NewGridWithColumns(3, noGen, withGen, faded),
	))
	w.Resize(geo.NewSize(620, 260))
	anim.Start()

	if *auto {
		go runAuto(a, w, map[string]core.CanvasObject{
			"SetPixels only":      noGen,
			"SetPixels+Generator": withGen,
			"Translucency 0.5":    faded,
		}, noGen, faded)
	}
	bellui.ShowAndRun(w)
}
