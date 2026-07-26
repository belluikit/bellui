// Command webgpuwaylandprobe verifies that the default (WebGPU) painter creates
// and renders to a native Wayland surface (#608, a #542 follow-up).
//
// The WebGPU backend's Wayland surface descriptor
// (SurfaceDescriptorFromWaylandSurface, backend_webgpu_x11wayland.go, gated on
// build.IsWayland) is compile-checked in CI but never runtime-exercised there —
// CI runs under Xvfb (X11). This probe forces the Wayland platform and draws a
// distinctive frame so a screenshot proves a frame reached the compositor.
//
// Run on a native Wayland session (e.g. headless sway + lavapipe):
//
//	BELLUI_PLATFORM=wayland WGPU_BACKEND=vulkan go run ./hack/webgpuwaylandprobe
//
// It prints build.IsWayland (whether bellui resolved to the Wayland backend) once
// the driver is up, enables wgpu info logging (adapter/backend), and quits after
// a delay so a script can grim the output.
package main

import (
	"fmt"
	"image/color"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	intbuild "github.com/belluikit/bellui/internal/build"
	"github.com/belluikit/bellui/widget"
	"github.com/belluikit/webgpu/wgpu"
)

func main() {
	// Surface wgpu's own logs (adapter, backend, surface config) — bellui does not
	// enable them by default.
	wgpu.SetLogLevel(wgpu.LogLevelInfo)

	logPath := filepath.Join(os.TempDir(), "bellui-webgpuwayland.log")
	logf, _ := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	var sink io.Writer = os.Stderr
	if logf != nil {
		sink = io.MultiWriter(logf, os.Stderr)
	}
	logger := log.New(sink, "", log.LstdFlags)

	a := app.NewWithID("io.bellui.webgpuwaylandprobe")
	w := a.NewWindow("WebGPU Wayland Probe")

	// A distinctive, unmistakable frame: magenta background, a cyan rectangle,
	// and large text. Easy to confirm in a screenshot and to distinguish from a
	// blank/black failure.
	bg := canvas.NewRectangle(color.NRGBA{R: 220, G: 30, B: 200, A: 255})
	box := canvas.NewRectangle(color.NRGBA{R: 30, G: 220, B: 220, A: 255})
	box.SetMinSize(geo.NewSize(240, 120))
	label := widget.NewLabel("WebGPU × Wayland #608")

	w.SetContent(container.NewStack(bg, container.NewCenter(container.NewVBox(box, label))))
	w.Resize(geo.NewSize(480, 320))

	go func() {
		// Wait for the driver/GLFW to initialise so build.IsWayland is resolved.
		time.Sleep(2 * time.Second)
		msg := fmt.Sprintf("build.IsWayland=%v  BELLUI_PLATFORM=%q", intbuild.IsWayland, os.Getenv("BELLUI_PLATFORM"))
		logger.Println(msg)
		fmt.Fprintln(os.Stderr, "PROBE:", msg)
		if !intbuild.IsWayland {
			logger.Println("WARNING: bellui did NOT resolve to the Wayland backend (X11 path). Force BELLUI_PLATFORM=wayland and ensure no CSD-heuristic X11 fallback.")
		}
		time.Sleep(6 * time.Second)
		bellui.Do(func() { a.Quit() })
	}()

	fmt.Println("webgpuwaylandprobe: log at", logPath)
	bellui.ShowAndRun(w)
}
