//go:build linux && wpe

// Command wpeprobe is the #60 spike: can Linux host web content as a *texture*
// bellui composites, instead of as the hole punch every other platform uses?
//
// The other backends (macOS, iOS, Android, wasm) put an OS view *over* bellui's
// surface. That costs three things, all documented on widget.NativeViewHost:
// nothing bellui draws can appear above the web content, Canvas.Capture sees a
// hole where it is, and the render pipeline does not reach it. WPE WebKit is
// the WebKit port built for embedding into someone else's renderer, so on paper
// Linux could avoid all three by receiving frames and drawing them itself.
//
// This probe measures whether that actually works, in two stages:
//
//	stage 1  take frames as CPU buffers and push them through the existing
//	         streaming-raster path (canvas.Raster.Streaming + SetPixels, #34).
//	         Proves the architecture end to end, and prices the copy.
//	stage 2  take frames as EGLImages and import them as textures with no copy.
//	         This is the shippable form, and the open question was whether the
//	         import works under the WebGPU-default painter.
//
// Run on a Linux box with libwpe, WPEBackend-fdo and WPE WebKit installed:
//
//	go run ./hack/wpeprobe            # report
//	go run ./hack/wpeprobe -dump out  # also write the captured PNGs
package main

/*
#cgo pkg-config: wpe-webkit-1.0 wpebackend-fdo-1.0 wpe-1.0 egl glib-2.0 wayland-server
#include <stdlib.h>
#include "wpe.h"
*/
import "C"

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/driver/software"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

const (
	viewW, viewH = 640, 480

	// Vivid, fully opaque, and nothing like any bellui theme colour, so
	// "the web content reached the canvas" is decidable by counting pixels
	// rather than by eye.
	pageHTML = `<html><body style="margin:0;background:#ff00aa">
<div style="font:64px sans-serif;color:#ffffff;padding:40px">WPE</div>
</body></html>`
)

var dump = flag.String("dump", "", "directory to write captured PNGs to")

func main() {
	flag.Parse()

	fmt.Println("=== #60 WPE spike: web content as a texture, not a hole punch ===")

	html := C.CString(pageHTML)
	defer C.free(unsafe.Pointer(html))
	if rc := C.belluiWpeStart(C.int(viewW), C.int(viewH), html); rc != 0 {
		fmt.Printf("FAIL: could not start WPE (rc=%d)\n", rc)
		os.Exit(1)
	}
	defer C.belluiWpeStop()

	frame, serial := waitForFrame(15 * time.Second)
	if frame == nil {
		fmt.Println("FAIL: no frame produced in 15s")
		os.Exit(2)
	}

	reportExportKind()
	measureFrameCost()
	stage1(frame, serial)
}

// reportExportKind is the finding the whole spike turns on: a CPU buffer means
// stage 2 has nothing to import, however capable the GPU is.
func reportExportKind() {
	fmt.Printf("\n[export]  how the engine hands frames over\n")
	switch C.belluiWpeExportKind() {
	case 1:
		fmt.Println("  SHM (a CPU buffer)")
		fmt.Println("  => stage 2 (zero-copy EGLImage import) is NOT REACHABLE on this stack:")
		fmt.Println("     there are no EGLImages to import, so the wgpu interop question")
		fmt.Println("     the spike was meant to answer cannot even be posed here.")
	case 2:
		fmt.Println("  EGLImage (a GPU texture)")
		fmt.Println("  => stage 2 is reachable; the open question is the wgpu import.")
	default:
		fmt.Println("  nothing yet")
	}
}

// stage1 draws the web frame through bellui's own painter and then asks the two
// questions the hole punch answers badly: does Capture see the content, and can
// bellui draw over it?
func stage1(frame *image.RGBA, serial uint64) {
	fmt.Printf("\n[stage 1] frames through canvas.Raster.Streaming + SetPixels\n")
	fmt.Printf("  frame %d: %dx%d\n", serial, frame.Bounds().Dx(), frame.Bounds().Dy())

	raster := canvas.NewRaster(nil)
	raster.Streaming = true
	raster.SetPixels(frame.Pix, frame.Bounds().Dx(), frame.Bounds().Dy())

	// A bellui overlay sitting on top of the web content. Under the hole punch
	// this is occluded — it is the single most visible limitation of the
	// approach — so it is the sharpest test of whether this one is different.
	badge := canvas.NewRectangle(color.NRGBA{R: 0, G: 200, B: 80, A: 255})
	badge.Resize(geo.NewSize(160, 60))
	badge.Move(geo.NewPos(viewW-200, viewH-100))

	content := container.NewWithoutLayout(raster, badge)
	raster.Resize(geo.NewSize(viewW, viewH))

	test.NewApp()
	c := software.NewCanvas()
	c.SetContent(content)
	c.Resize(geo.NewSize(viewW, viewH))

	shot := software.RenderCanvas(c, theme.DefaultTheme())
	writePNG("stage1.png", shot)

	web := countNear(shot, color.NRGBA{R: 0xff, G: 0x00, B: 0xaa, A: 0xff}, 40)
	over := countNear(shot, color.NRGBA{R: 0, G: 200, B: 80, A: 255}, 40)
	total := shot.Bounds().Dx() * shot.Bounds().Dy()

	fmt.Printf("\n[capture] what bellui's own Capture() sees\n")
	fmt.Printf("  web-content pixels   %7d / %d  (%.1f%%)\n", web, total, pct(web, total))
	fmt.Printf("  bellui overlay above %7d / %d  (%.1f%%)\n", over, total, pct(over, total))

	switch {
	case web == 0:
		fmt.Println("  FAIL: the web content never reached bellui's surface")
	case over == 0:
		fmt.Println("  FAIL: bellui could not draw above the web content")
	default:
		fmt.Println("  PASS: the content is IN the canvas and bellui draws OVER it —")
		fmt.Println("        both impossible under the hole punch the other platforms use.")
	}
}

// measureFrameCost prices the CPU roundtrip, so whether stage 1 is shippable is
// a measurement rather than an assumption. It varies run to run — the copy
// contends with the engine thread still producing frames — so treat the figures
// as a range and read the extrapolation, not the single sample.
func measureFrameCost() {
	buf := make([]byte, viewW*viewH*4)
	var w, h, stride C.int

	const runs = 30
	start := time.Now()
	for range runs {
		C.belluiWpeFrame((*C.uchar)(unsafe.Pointer(&buf[0])), C.int(len(buf)), &w, &h, &stride)
		swizzleBGRA(buf)
	}
	per := time.Since(start) / runs

	fmt.Printf("\n[cost]    CPU roundtrip per frame (copy out + BGRA->RGBA)\n")
	const budget = 16600 * time.Microsecond // one frame at 60fps
	fmt.Printf("  %v/frame at %dx%d  =>  %.1f%% of a 16.6ms frame budget\n",
		per.Round(time.Microsecond), viewW, viewH, float64(per)/float64(budget)*100)
	// The cost is per pixel, so the number that decides shippability is the one
	// at a real window size, not at the probe's.
	for _, s := range []struct {
		name string
		w, h int
	}{
		{"720p", 1280, 720}, {"1080p", 1920, 1080}, {"1440p", 2560, 1440},
	} {
		scaled := time.Duration(float64(per) * float64(s.w*s.h) / float64(viewW*viewH))
		fmt.Printf("  extrapolated %-6s %8v/frame  =>  %5.1f%% of budget\n",
			s.name, scaled.Round(time.Microsecond), float64(scaled)/float64(budget)*100)
	}
}

func waitForFrame(timeout time.Duration) (*image.RGBA, uint64) {
	buf := make([]byte, viewW*viewH*4)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var w, h, stride C.int
		serial := C.belluiWpeFrame((*C.uchar)(unsafe.Pointer(&buf[0])), C.int(len(buf)), &w, &h, &stride)
		if serial > 0 && w > 0 && h > 0 {
			img := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
			copy(img.Pix, buf[:int(stride)*int(h)])
			swizzleBGRA(img.Pix)
			return img, uint64(serial)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, 0
}

// swizzleBGRA converts the engine's byte order to Go's. WPE hands over
// ARGB8888 little-endian, which is B,G,R,A in memory; image.RGBA wants R,G,B,A.
func swizzleBGRA(pix []byte) {
	for i := 0; i+3 < len(pix); i += 4 {
		pix[i], pix[i+2] = pix[i+2], pix[i]
	}
}

// countNear counts pixels within tolerance of want, which is how "did this
// colour survive into the capture" is decided without depending on exact
// blending or gamma.
func countNear(img image.Image, want color.NRGBA, tolerance int) int {
	b := img.Bounds()
	n := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bb, _ := img.At(x, y).RGBA()
			if near(int(r>>8), int(want.R), tolerance) &&
				near(int(g>>8), int(want.G), tolerance) &&
				near(int(bb>>8), int(want.B), tolerance) {
				n++
			}
		}
	}
	return n
}

func near(a, b, tol int) bool { return a-b <= tol && b-a <= tol }

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}

func writePNG(name string, img image.Image) {
	if *dump == "" {
		return
	}
	if err := os.MkdirAll(*dump, 0o755); err != nil {
		bellui.LogError("wpeprobe: mkdir", err)
		return
	}
	path := filepath.Join(*dump, name)
	f, err := os.Create(path)
	if err != nil {
		bellui.LogError("wpeprobe: create", err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		bellui.LogError("wpeprobe: encode", err)
		return
	}
	fmt.Printf("  wrote %s\n", path)
}
