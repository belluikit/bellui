// Command paintcullprobe is the on-device check for the mobile paint walk's
// off-clip culling (#481, the desktop half was #464). It runs perfscenes'
// spinner scene, n cards in a scroll container plus one infinite progress bar,
// and prints to stdout:
//
//	RESULT scene=spinner n=... frames=... fps=... cpu_ms_per_frame=...
//	CAPTURE offset=... size=WxH sha256=...
//
// The RESULT line is frame cost, measured as perfscenes measures it: one
// refresh per painted frame, after a warm-up, with cpu_ms_per_frame counting
// every thread; a CPU profile of that window goes to the app's temporary
// directory (PROFILE line). Then the spinner is hidden and the window is
// captured at a series of scroll offsets, some leaving a card edge inside the
// cull margin; build the probe before and after a change and the hashes must
// match.
//
// Build and run on an iOS device (stdout streams back with --console):
//
//	bellui package -os ios -app-id com.trendvidia.paintcullprobe -src hack/paintcullprobe
//	xcrun devicectl device install app --device <id> paintcullprobe.app
//	xcrun devicectl device process launch --console --terminate-existing --device <id> com.trendvidia.paintcullprobe
//
// On Android, build with -os android and read stdout with adb logcat. On a
// desktop, go run ./hack/paintcullprobe opens a 1920×1080 window.
package main

import (
	"crypto/sha256"
	"fmt"
	"image"
	"image/draw"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"syscall"
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
	warm = 3 * time.Second
	secs = 10 * time.Second
)

// cards is the number of cards, 500 unless PAINTCULL_N says otherwise
// (devicectl launch --environment-variables '{"PAINTCULL_N":"2000"}').
var cards = 500

// offsets are the scroll positions captured. Rows are 200 high plus padding,
// so several of these leave a card edge just outside the view, inside the
// 32 px cull margin.
var offsets = []float32{0, 37, 199, 230, 1000, 2345, 4096, 6000}

var painted = make(chan time.Time, 1)

func main() {
	if n, err := strconv.Atoi(os.Getenv("PAINTCULL_N")); err == nil && n > 0 {
		cards = n
	}
	a := app.NewWithID("com.trendvidia.paintcullprobe")
	w := a.NewWindow("paintcullprobe")
	mark := canvas.NewRaster(func(int, int) image.Image {
		select {
		case painted <- time.Now():
		default:
		}
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	})
	objs := make([]core.CanvasObject, cards)
	for i := range objs {
		body := container.NewVBox(widget.NewLabel(strconv.Itoa(i)), widget.NewProgressBar(), widget.NewButton("Open", func() {}))
		objs[i] = widget.NewCard("Tile "+strconv.Itoa(i), "", body)
	}
	scroll := container.NewVScroll(container.NewGridWrap(geo.NewSize(180, 200), objs...))
	spinner := widget.NewProgressBarInfinite()
	w.SetContent(container.NewBorder(container.NewHBox(mark, spinner), nil, nil, nil, scroll))
	if !bellui.CurrentDevice().IsMobile() {
		w.Resize(geo.NewSize(1920, 1080)) // perfscenes' window; a phone's is the screen
	}

	go func() {
		if err := run(w, mark, spinner, scroll); err != nil {
			fmt.Println("ERROR", err)
			os.Exit(1)
		}
		fmt.Println("DONE")
		os.Exit(0)
	}()
	bellui.ShowAndRun(w)
}

func run(w bellui.Window, mark *canvas.Raster, spinner *widget.ProgressBarInfinite, scroll *container.Scroll) error {
	time.Sleep(time.Second)
	if err := measure(mark); err != nil {
		return err
	}
	// The stopped spinner's pose varies from run to run; hide it so the
	// captures show only what the paint walk decides.
	bellui.DoAndWait(func() { spinner.Stop(); spinner.Hide() })
	for _, off := range offsets {
		bellui.DoAndWait(func() { scroll.ScrollToOffset(geo.NewPos(0, off)) })
		// Two painted frames: the scroll's own refresh, then a settled one.
		for range 2 {
			if err := nextFrame(mark); err != nil {
				return err
			}
		}
		var img image.Image
		bellui.DoAndWait(func() { img = w.Canvas().Capture() })
		if img == nil {
			return fmt.Errorf("capture at offset %v returned nothing", off)
		}
		rgba := image.NewRGBA(img.Bounds())
		draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)
		b := rgba.Bounds()
		fmt.Printf("CAPTURE offset=%v size=%dx%d sha256=%x\n", off, b.Dx(), b.Dy(), sha256.Sum256(rgba.Pix))
	}
	return nil
}

// nextFrame refreshes the mark and waits until a frame painted it.
func nextFrame(mark *canvas.Raster) error {
	bellui.DoAndWait(mark.Refresh)
	select {
	case <-painted:
		return nil
	case <-time.After(2 * time.Second):
		return fmt.Errorf("no frame painted within 2s")
	}
}

// measure paints one refresh per frame, warms up, then measures for secs and
// prints the RESULT line.
func measure(mark *canvas.Raster) error {
	warmEnd := time.Now().Add(warm)
	for time.Now().Before(warmEnd) {
		if err := nextFrame(mark); err != nil {
			return err
		}
	}
	prof, err := os.Create(filepath.Join(os.TempDir(), "paintcull.cpu.prof"))
	if err != nil {
		return err
	}
	if err := pprof.StartCPUProfile(prof); err != nil {
		return err
	}
	start, cpuStart := time.Now(), cpuTime()
	n := 0
	for time.Since(start) < secs {
		if err := nextFrame(mark); err != nil {
			return err
		}
		n++
	}
	wall, cpu := time.Since(start), cpuTime()-cpuStart
	pprof.StopCPUProfile()
	if err := prof.Close(); err != nil {
		return err
	}
	fmt.Println("PROFILE", prof.Name())
	fmt.Printf("RESULT scene=spinner n=%d frames=%d fps=%.1f cpu_ms_per_frame=%.2f\n",
		cards, n, float64(n)/wall.Seconds(), float64(cpu.Microseconds())/1000/float64(n))
	return nil
}

// cpuTime is the process's user plus system time, across all threads.
func cpuTime() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}
