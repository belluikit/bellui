//go:build linux && cef

// Command cefprobe is the #60 phase 0 spike (belluikit/bellui#102): can Linux host web
// content as a *texture bellui composites*, with CEF as the engine?
//
// It is the direct successor to hack/wpeprobe. That probe established the
// architecture — engine frames pushed through canvas.Raster.Streaming +
// SetPixels land inside bellui's canvas, with bellui drawing over them — and
// then rejected WPE as the engine, on grounds that had nothing to do with the
// architecture: 112 unbundled dependencies, absent from Ubuntu since 22.04,
// zero-copy conditional on the user's session type. CEF replaces the engine,
// not the approach, so this probe deliberately reuses wpeprobe's assertions and
// prints them in the same shape, and the two reports can be read side by side.
//
// What it answers that wpeprobe could not:
//
//  1. does CEF initialize headless, with an external pump and a helper
//     subprocess, at all
//  2. do frames arrive through on_paint and reach bellui's surface
//  3. what does the CPU roundtrip cost per frame, extrapolated to 1080p
//  4. does on_accelerated_paint fire on Linux, and what do its dma-buf planes
//     carry — the zero-copy question WPEDisplayHeadless could not even pose,
//     since it exposes no DRM device and so can only ever hand over SHM
//
// Setup, and the payload it needs, is in README.md. Behind the `cef` build tag,
// so `go build ./...` on a machine with no CEF payload — which is every CI
// runner — does not try to compile it.
package main

/*
#cgo CFLAGS: -I${SRCDIR} -I${SRCDIR}/cefroot
#cgo LDFLAGS: -ldl
#include <stdlib.h>
#include "cef.h"
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
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/driver/software"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// The probe page is a solid, unmistakable colour, so counting pixels is a
// sufficient test of "did the web content actually arrive". Same value
// wpeprobe used, so the two reports compare directly.
const probePage = `data:text/html,<body style="margin:0;background:%23ff00aa">`

var (
	webColor     = color.NRGBA{R: 0xff, G: 0x00, B: 0xaa, A: 0xff}
	overlayColor = color.NRGBA{R: 0, G: 200, B: 80, A: 255}
)

var (
	url    = flag.String("url", probePage, "page to load")
	viewW  = flag.Int("w", 800, "view width in pixels")
	viewH  = flag.Int("h", 600, "view height in pixels")
	cefDir = flag.String("cef", "", "directory holding libcef.so and Resources (default: ./cefroot)")
	dump   = flag.String("dump", "", "directory to write captured PNGs to")
	extra  = flag.String("switches", "", "extra Chromium switches, space separated")
	accel  = flag.Bool("accel", true, "also probe on_accelerated_paint (shared textures)")
)

// frames carries what the paint callbacks report. CEF calls them on the thread
// that pumps the message loop, which is this program's main thread, but the
// mutex is kept anyway: the whole point of the seam is that a real backend may
// read the last frame from elsewhere.
var frames struct {
	sync.Mutex
	pix     []byte // RGBA, already swizzled
	w, h    int
	serial  uint64
	popups  int
	accel   accelReport
	accelOK bool
}

type accelReport struct {
	planeCount int
	fd         int
	stride     uint32
	modifier   uint64
	format     int
}

//export belluiCefPaint
func belluiCefPaint(buffer unsafe.Pointer, w, h C.int, isPopup C.int) {
	n := int(w) * int(h) * 4
	src := unsafe.Slice((*byte)(buffer), n)

	frames.Lock()
	defer frames.Unlock()
	if isPopup != 0 {
		// A <select> dropdown renders as its own surface. The probe only counts
		// them; compositing the popup rect over the view is the backend's job
		// (belluikit/bellui#105), and skipping it is why dropdowns are invisible
		// in naive OSR integrations.
		frames.popups++
		return
	}
	if cap(frames.pix) < n {
		frames.pix = make([]byte, n)
	}
	frames.pix = frames.pix[:n]
	copy(frames.pix, src)
	swizzleBGRA(frames.pix)
	frames.w, frames.h = int(w), int(h)
	frames.serial++
}

//export belluiCefAccelPaint
func belluiCefAccelPaint(planeCount, fd C.int, stride C.uint, modifier C.ulonglong, format C.int) {
	frames.Lock()
	defer frames.Unlock()
	frames.accelOK = true
	frames.accel = accelReport{
		planeCount: int(planeCount),
		fd:         int(fd),
		stride:     uint32(stride),
		modifier:   uint64(modifier),
		format:     int(format),
	}
	frames.serial++
}

func main() {
	// CEF's browser UI thread is whichever thread called cef_initialize, and
	// every subsequent call — cef_do_message_loop_work above all — has to come
	// from that same thread. Go does not promise that: the main goroutine may be
	// rescheduled onto another OS thread at any preemption point, and this
	// probe's pump loop sleeps on every iteration, which is exactly such a
	// point.
	//
	// Without this lock the probe delivered a first frame on roughly one run in
	// three, with byte-identical logs whether it worked or not. It reads like a
	// CEF or GPU problem and is neither.
	//
	// bellui's own main loop already runs locked to the main thread, which is
	// why the backend (belluikit/bellui#105) inherits this for free — but only
	// as long as every CEF call goes through bellui.Do rather than a goroutine.
	runtime.LockOSThread()

	flag.Parse()

	// Default to wherever setup.sh staged the payload, for both of the ways
	// this gets run: `go run ./hack/cefprobe` from the repo root, and directly
	// from the probe's own directory.
	dir := *cefDir
	if dir == "" {
		wd, _ := os.Getwd()
		for _, candidate := range []string{
			filepath.Join(wd, "hack", "cefprobe", "cefroot"),
			filepath.Join(wd, "cefroot"),
		} {
			if _, err := os.Stat(filepath.Join(candidate, "libcef.so")); err == nil {
				dir = candidate
				break
			}
		}
		if dir == "" {
			fmt.Println("  no payload found — run hack/cefprobe/setup.sh first")
			os.Exit(1)
		}
	}

	fmt.Println("== #60 phase 0: CEF offscreen rendering into bellui's canvas ==")
	fmt.Printf("  payload   %s\n", dir)
	fmt.Printf("  view      %dx%d\n", *viewW, *viewH)

	if !start(dir, false) {
		os.Exit(1)
	}

	frame, serial := waitForFrame(20 * time.Second)
	if frame == nil {
		fmt.Println("\n  FAIL: no frame arrived through on_paint within 20s")
		C.belluiCefShutdown()
		os.Exit(1)
	}

	composite(frame, serial)
	measureFrameCost()
	reportPopups()

	C.belluiCefShutdown()

	if *accel {
		probeAccelerated(dir)
	}
}

// start brings CEF up and creates the windowless browser.
func start(dir string, sharedTexture bool) bool {
	cdir := C.CString(dir)
	defer C.free(unsafe.Pointer(cdir))
	helper := C.CString(filepath.Join(dir, "cefprobe_helper"))
	defer C.free(unsafe.Pointer(helper))
	sw := C.CString(switches())
	defer C.free(unsafe.Pointer(sw))

	if C.belluiCefInit(cdir, helper, sw) != 0 {
		fmt.Printf("\n  FAIL: %s\n", C.GoString(C.belluiCefError()))
		return false
	}

	curl := C.CString(*url)
	defer C.free(unsafe.Pointer(curl))
	shared := C.int(0)
	if sharedTexture {
		shared = 1
	}
	if C.belluiCefCreateBrowser(curl, C.int(*viewW), C.int(*viewH), shared) != 0 {
		fmt.Printf("\n  FAIL: %s\n", C.GoString(C.belluiCefError()))
		return false
	}
	return true
}

// switches assembles the Chromium command line. On a machine with no display
// server — an SSH session on the GPU box, or CI — Chromium's ozone layer still
// tries X11 and exits with "Missing X server or $DISPLAY" before any of this
// gets interesting, so headless is selected explicitly. wpeprobe had the same
// shape of problem and solved it the same way, by setting WPE_BACKEND_LIBRARY
// itself rather than making every caller remember.
//
// Note this says nothing about GPU access: an offscreen browser still reaches
// the GPU process, which is the whole reason the accelerated path is worth
// probing on a box with a real card and no desktop session.
func switches() string {
	s := *extra
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		if s != "" {
			s += " "
		}
		s += "--ozone-platform=headless"
		fmt.Println("  session   headless (no DISPLAY/WAYLAND_DISPLAY) → --ozone-platform=headless")
	}
	return s
}

// composite is the assertion that matters, and it is wpeprobe's verbatim: does
// bellui's own Capture() see the web content, and can bellui draw above it?
// Both are impossible under the hole punch every other platform uses.
func composite(frame *image.RGBA, serial uint64) {
	fmt.Printf("\n[compose] frames through canvas.Raster.Streaming + SetPixels\n")
	fmt.Printf("  frame %d: %dx%d\n", serial, frame.Bounds().Dx(), frame.Bounds().Dy())

	raster := canvas.NewRaster(nil)
	raster.Streaming = true
	raster.SetPixels(frame.Pix, frame.Bounds().Dx(), frame.Bounds().Dy())

	badge := canvas.NewRectangle(overlayColor)
	badge.Resize(geo.NewSize(160, 60))
	badge.Move(geo.NewPos(float32(*viewW-200), float32(*viewH-100)))

	content := container.NewWithoutLayout(raster, badge)
	raster.Resize(geo.NewSize(float32(*viewW), float32(*viewH)))

	test.NewApp()
	c := software.NewCanvas()
	c.SetContent(content)
	c.Resize(geo.NewSize(float32(*viewW), float32(*viewH)))

	shot := software.RenderCanvas(c, theme.DefaultTheme())
	writePNG("compose.png", shot)

	web := countNear(shot, webColor, 40)
	over := countNear(shot, overlayColor, 40)
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

// measureFrameCost prices the CPU roundtrip — the copy plus the BGRA→RGBA
// swizzle — so whether this path is shippable is a measurement rather than an
// assumption. wpeprobe measured 0.4–1.5 ms at 640x480 for the copy alone,
// extrapolating to 3–9 ms at 1080p, which is 17–59% of a frame budget.
func measureFrameCost() {
	frames.Lock()
	w, h := frames.w, frames.h
	frames.Unlock()
	if w == 0 || h == 0 {
		return
	}

	src := make([]byte, w*h*4)
	dst := make([]byte, w*h*4)

	const runs = 60
	start := time.Now()
	for i := 0; i < runs; i++ {
		copy(dst, src)
		swizzleBGRA(dst)
	}
	per := time.Since(start) / runs

	// Scale by pixel count: the work is linear in pixels.
	scale := float64(1920*1080) / float64(w*h)
	at1080 := time.Duration(float64(per) * scale)

	fmt.Printf("\n[cost] CPU roundtrip per frame (copy + BGRA→RGBA swizzle)\n")
	fmt.Printf("  at %dx%d      %v\n", w, h, per)
	fmt.Printf("  at 1920x1080   %v  (%.0f%% of a 16.6ms frame budget)\n",
		at1080, float64(at1080)/float64(16600*time.Microsecond)*100)
	fmt.Println("  note: the swizzle belongs in the shader, not in Go — see belluikit/bellui#105.")
}

func reportPopups() {
	frames.Lock()
	n := frames.popups
	frames.Unlock()
	fmt.Printf("\n[popups] PET_POPUP surfaces seen: %d\n", n)
	if n == 0 {
		fmt.Println("  none on this page; a <select> dropdown produces them, and a backend")
		fmt.Println("  that ignores them renders dropdowns invisible (belluikit/bellui#105).")
	}
}

// probeAccelerated restarts CEF asking for shared textures, which is the whole
// zero-copy question. On Linux the info carries dma-buf fds; whether those can
// be imported into the WebGPU painter is a separate matter (risk R1 on #60),
// but they have to exist first.
func probeAccelerated(dir string) {
	fmt.Printf("\n[accel] on_accelerated_paint (shared_texture_enabled = 1)\n")

	frames.Lock()
	frames.accelOK = false
	frames.serial = 0
	frames.Unlock()

	if !start(dir, true) {
		fmt.Println("  FAIL: could not restart CEF with shared textures")
		return
	}
	defer C.belluiCefShutdown()

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		C.belluiCefWork()
		frames.Lock()
		ok, rep := frames.accelOK, frames.accel
		frames.Unlock()
		if ok {
			fmt.Printf("  PASS: dma-buf frame delivered\n")
			fmt.Printf("    planes    %d\n", rep.planeCount)
			fmt.Printf("    fd        %d\n", rep.fd)
			fmt.Printf("    stride    %d\n", rep.stride)
			fmt.Printf("    modifier  0x%x\n", rep.modifier)
			fmt.Printf("    format    %d\n", rep.format)
			fmt.Println("  zero-copy is reachable in principle; importing into the WebGPU")
			fmt.Println("  painter is the open question (risk R1 on #60).")
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	fmt.Println("  no accelerated frame within 20s — the CPU path is the shippable one.")
	fmt.Println("  (headless with no GPU process falls back to on_paint by design.)")
}

// waitForFrame pumps CEF until a frame lands.
func waitForFrame(timeout time.Duration) (*image.RGBA, uint64) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		C.belluiCefWork()

		frames.Lock()
		if frames.serial > 0 && frames.w > 0 {
			img := image.NewRGBA(image.Rect(0, 0, frames.w, frames.h))
			copy(img.Pix, frames.pix)
			serial := frames.serial
			frames.Unlock()
			return img, serial
		}
		frames.Unlock()

		time.Sleep(5 * time.Millisecond)
	}
	return nil, 0
}

// swizzleBGRA converts CEF's BGRA buffer to the RGBA canvas.Raster wants, in
// place. A real backend does this in the shader — it is here so the measured
// cost is honest about what the CPU path actually involves.
func swizzleBGRA(pix []byte) {
	for i := 0; i+3 < len(pix); i += 4 {
		pix[i], pix[i+2] = pix[i+2], pix[i]
	}
}

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
		fmt.Println("  dump:", err)
		return
	}
	path := filepath.Join(*dump, name)
	f, err := os.Create(path)
	if err != nil {
		fmt.Println("  dump:", err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		fmt.Println("  dump:", err)
		return
	}
	fmt.Printf("  wrote %s\n", path)
}
