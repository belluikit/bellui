package main

import (
	"fmt"
	"image"
	"os"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/geo"
)

// This is the half of the probe that has to be written per platform.
//
// Everything -auto measures rests on one trick: capture the same window twice,
// once through bellui (Canvas.Capture, which reads bellui's own GPU surface)
// and once through the operating system (which reads the real composited
// screen, and therefore does include the native view). The difference between
// the two *is* the hole punch, which turns each of the host's documented
// limitations into a number instead of an opinion.
//
// Only the second capture is platform-specific, so only it lives behind a build
// tag. A platform with no implementation makes -auto say so and stop, rather
// than silently measuring bellui's surface against itself and reporting that
// everything passes.

// screenshotRegion is the window area to capture, in physical pixels on the
// screen — the same rectangle on every platform, resolved from the window
// position (#497) and the canvas scale.
type screenshotRegion struct {
	x, y, w, h int
}

// screenshot grabs the window's content area from the composited screen and
// decodes it, writing the intermediate file to path so that -dump can keep it.
func screenshot(w bellui.Window, path string) (image.Image, error) {
	r, err := windowRegion(w)
	if err != nil {
		return nil, err
	}
	if err := captureScreen(r, path); err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	return img, err
}

func windowRegion(w bellui.Window) (screenshotRegion, error) {
	var x, y int
	var size geo.Size
	var scale float32
	bellui.DoAndWait(func() {
		if pw, ok := w.(desktop.PositionableWindow); ok {
			x, y = pw.Position()
		}
		size = w.Canvas().Size()
		scale = w.Canvas().Scale()
	})
	if size.IsZero() {
		return screenshotRegion{}, fmt.Errorf("window has no size yet")
	}

	r := screenshotRegion{x: x, y: y, w: int(size.Width * scale), h: int(size.Height * scale)}
	if *dumpDir != "" {
		fmt.Printf("            [dbg] window pos (%d,%d) canvas %v scale %.2f -> region %+v\n",
			x, y, size, scale, r)
	}
	return r, nil
}
