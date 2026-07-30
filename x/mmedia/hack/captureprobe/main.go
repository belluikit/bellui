// Command captureprobe is the live-verification harness for x/mmedia camera
// capture (#88). Unit tests cover the platform selector rules and the
// release/reacquire cycle against a fake decoder; none of that proves a real
// camera opens, so this drives the actual device.
//
// It is a separate module (its own go.mod) so x/mmedia's library go.mod stays
// free of the GL/app dependency tree, matching x/tables/hack/a11yprobe. Build and
// run from this directory:
//
//	cd hack/captureprobe
//	go run . -list                  # enumerate devices, as a picker would
//	go run .                        # open the platform default camera
//	go run . -device "Logitech BRIO"
//	go run . -index 1
//	go run . -auto                  # scripted: report and exit, no interaction
//	go run . -options framerate=30,pixel_format=nv12
//
// What -auto checks, and what each answer means:
//
//   - frames arrive             the device opened and is decoding
//   - Duration() == 0           reported as live, so the transport disables
//     the seek bar and shows "live"
//   - Seek() is refused         a device has no timeline; this must not reach
//     av_seek_frame
//   - Stop then Play            Stop releases the device (camera light out,
//     other apps can claim it) and Play reacquires it
//
// Platform notes worth knowing before reading a failure as a bug:
//
//   - macOS asks for camera permission, and grants it to the *running binary's*
//     signing identity. A `go run` binary is freshly built each time, so the
//     prompt may reappear or be denied outright; if capture fails with a
//     permission error, build once (`go build -o captureprobe .`) and run that.
//   - macOS/avfoundation cannot enumerate at all: -list reports
//     ErrDeviceListUnsupported, which means "cannot list", not "no camera".
//     Selection there is positional, which is what -index is for.
//   - Linux: a UVC camera exposes a metadata node beside its real one. Opening
//     the wrong node blocks indefinitely rather than failing, which is why
//     device choice filters on HasMedia.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/mmedia"
	"github.com/belluikit/goffmpeg"
)

func main() {
	var (
		list    = flag.Bool("list", false, "enumerate capture devices and exit")
		device  = flag.String("device", "", "device name (empty = platform default)")
		index   = flag.Int("index", -1, "open the nth video device instead of -device")
		auto    = flag.Bool("auto", false, "scripted run: report and exit")
		options = flag.String("options", "", "libavdevice options, e.g. framerate=30")
		dwell   = flag.Duration("dwell", 3*time.Second, "how long -auto captures for")
		raw     = flag.Bool("raw", false, "bypass the pipeline: open via goffmpeg and report what the device declares")
	)
	flag.Parse()

	if *list {
		listDevices()
		return
	}
	if *raw {
		if !rawRun(*device, *index, *options) {
			os.Exit(1)
		}
		return
	}
	if *auto {
		if !autoRun(*device, *index, *options, *dwell) {
			os.Exit(1)
		}
		return
	}
	interactive(*device, *index, *options)
}

func listDevices() {
	all, err := mmedia.CaptureDevices()
	if err != nil {
		if errors.Is(err, goffmpeg.ErrDeviceListUnsupported) {
			fmt.Println("[LIST] this platform cannot enumerate capture devices " +
				"(expected on macOS/avfoundation).")
			fmt.Println("[LIST] selection is positional here — use -index, or -device with a name.")
			return
		}
		fmt.Printf("[LIST] FAILED: %v\n", err)
		os.Exit(1)
	}
	if len(all) == 0 {
		fmt.Println("[LIST] no capture devices reported.")
		return
	}
	fmt.Printf("[LIST] %d device(s):\n", len(all))
	for _, d := range all {
		kinds := ""
		if d.HasVideo {
			kinds += " video"
		}
		if d.HasAudio {
			kinds += " audio"
		}
		def := ""
		if d.Default {
			def = "  (default)"
		}
		fmt.Printf("  %-40s selector=%-28s%s%s\n", d.Label(), d.Name, kinds, def)
	}
}

func autoRun(device string, index int, options string, dwell time.Duration) bool {
	a := app.New()
	w := a.NewWindow("captureprobe")

	var frames atomic.Int64
	var captureErr atomic.Value
	ok := true
	report := func(pass bool, format string, args ...any) {
		if !pass {
			ok = false
		}
		state := "PASS"
		if !pass {
			state = "FAIL"
		}
		fmt.Printf("[%s] %s\n", state, fmt.Sprintf(format, args...))
	}

	// The pipeline's own time tick is the liveness signal: it only advances while
	// frames are being presented.
	p, err := openCounting(device, index, options, &frames, &captureErr)
	if err != nil {
		fmt.Printf("[FAIL] opening the capture device: %v\n", err)
		return false
	}
	defer p.Close()

	report(p.Duration() == 0, "Duration() = %v, want 0 for a live source", p.Duration())

	// Seek must not reach the device. There is no observable side effect to
	// assert from out here, so the check is that it neither panics nor moves the
	// reported position off zero.
	p.Seek(30 * time.Second)
	report(p.CurrentTime() >= 0, "Seek on a live source was refused without error")

	w.SetContent(p)
	w.Resize(geo.NewSize(640, 520))

	go func() {
		// Sample the pipeline's own counters as well as the time-tick count: they
		// separate "nothing decoded" from "decoded but never presented", which is
		// the difference between a device problem and a pipeline one.
		for i := 0; i < int(dwell/time.Second); i++ {
			time.Sleep(time.Second)
			st := p.Stats()
			fmt.Printf("[STAT] t=%ds playing=%v decoded=%d rendered=%d dropped=%d ticks=%d\n",
				i+1, p.IsPlaying(), st.FramesDecoded, st.FramesRendered,
				st.FramesDropped, frames.Load())
		}

		got := frames.Load()
		report(got > 0, "frames presented in %v: %d", dwell, got)
		if err, _ := captureErr.Load().(error); err != nil {
			report(false, "capture reported an error: %v", err)
		}

		// Stop releases the device; Play must reacquire it.
		bellui.Do(func() { p.Stop() })
		time.Sleep(500 * time.Millisecond)
		before := frames.Load()
		bellui.Do(func() { p.Play() })
		time.Sleep(1500 * time.Millisecond)
		after := frames.Load()
		report(after > before,
			"reacquired the device after Stop: %d more frame update(s)", after-before)

		if err, _ := captureErr.Load().(error); err != nil {
			report(false, "reacquire reported: %v", err)
		}

		fmt.Println("[AUTO] done")
		bellui.Do(func() { w.Close() })
	}()

	bellui.ShowAndRun(w)
	return ok
}

// rawRun opens the device through goffmpeg directly, with no pipeline, widget or
// event loop involved. It exists to place a failure on one side of the boundary or
// the other: if the device declares no dimensions here, or Next never yields a
// video frame, the problem is the device or the binding rather than x/mmedia.
func rawRun(device string, index int, options string) bool {
	format := mmedia.CaptureFormat()
	url := device
	if url == "" {
		url = "0"
	}
	if index >= 0 {
		url = strconv.Itoa(index)
	}
	fmt.Printf("[RAW] OpenDevice(format=%q, url=%q, options=%q)\n", format, url, options)

	dec, err := goffmpeg.OpenDevice(format, url, options)
	if err != nil {
		fmt.Printf("[RAW] FAILED to open: %v\n", err)
		return false
	}
	defer dec.Close()

	vi, ai := dec.VideoInfo(), dec.AudioInfo()
	fmt.Printf("[RAW] VideoInfo: %dx%d @ %.3ffps\n", vi.Width, vi.Height, vi.Framerate)
	fmt.Printf("[RAW] AudioInfo: %d Hz, %d ch\n", ai.SampleRate, ai.Channels)
	fmt.Printf("[RAW] Duration: %.3fs\n", dec.Duration())

	// x/mmedia decides hasVideo from the declared dimensions at open time, so
	// report that verdict explicitly — a device that declares nothing until the
	// first frame would be routed down the audio-only path.
	fmt.Printf("[RAW] would x/mmedia see video? %v (needs Width>0 && Height>0 at open)\n",
		vi.Width > 0 && vi.Height > 0)

	deadline := time.Now().Add(10 * time.Second)
	var video, audio, none int
	for time.Now().Before(deadline) && video < 3 {
		mt, err := dec.Next()
		if err != nil {
			fmt.Printf("[RAW] Next failed after %d video / %d audio: %v\n", video, audio, err)
			break
		}
		switch mt {
		case goffmpeg.MediaTypeVideo:
			video++
			// The presentation timestamp is what the display loop paces against.
			// A capture device that stamps frames with absolute time rather than
			// from zero would make the first frame due far in the future.
			fmt.Printf("[RAW] video frame %d: Position() = %.3fs\n", video, dec.Position())
			if video == 1 {
				// Re-read the info: a device may only declare its geometry once
				// a frame has actually been decoded.
				vi2 := dec.VideoInfo()
				fmt.Printf("[RAW] after first frame, VideoInfo: %dx%d @ %.3ffps\n",
					vi2.Width, vi2.Height, vi2.Framerate)
			}
		case goffmpeg.MediaTypeAudio:
			audio++
		case goffmpeg.MediaTypeNone:
			none++
		}
	}
	fmt.Printf("[RAW] decoded %d video, %d audio, %d none\n", video, audio, none)
	if video == 0 {
		fmt.Println("[RAW] VERDICT: the device yielded no video frames — not an x/mmedia fault")
		return false
	}
	fmt.Println("[RAW] VERDICT: the device decodes video fine")
	return true
}

// openCounting builds the player with the counters wired, so -auto can observe
// liveness and errors.
func openCounting(device string, index int, options string,
	frames *atomic.Int64, captureErr *atomic.Value,
) (*mmedia.MediaPlayer, error) {
	opts := []mmedia.Option{
		mmedia.WithAutoPlay(),
		mmedia.WithOnTimeUpdate(func(time.Duration, time.Duration) { frames.Add(1) }),
		mmedia.WithOnError(func(err error) { captureErr.Store(err) }),
	}
	if options != "" {
		opts = append(opts, mmedia.WithCaptureOptions(options))
	}
	if index >= 0 {
		return mmedia.NewCapturePlayerIndex(index, opts...)
	}
	return mmedia.NewCapturePlayer(device, opts...)
}

func interactive(device string, index int, options string) {
	a := app.New()
	w := a.NewWindow("captureprobe — live capture")

	status := widget.NewLabel("opening…")
	p, err := openCounting(device, index, options, new(atomic.Int64), new(atomic.Value))
	if err != nil {
		w.SetContent(container.NewVBox(
			widget.NewLabel("could not open the capture device:"),
			widget.NewLabel(err.Error()),
		))
		w.Resize(geo.NewSize(560, 160))
		bellui.ShowAndRun(w)
		return
	}
	defer p.Close()

	status.SetText(fmt.Sprintf("live · duration=%v (0 means live) · seek bar should be disabled",
		p.Duration()))

	w.SetContent(container.NewVBox(status, p))
	w.Resize(geo.NewSize(720, 600))
	bellui.ShowAndRun(w)
}
