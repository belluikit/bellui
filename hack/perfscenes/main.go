//go:build darwin || linux

// perfscenes measures whole-app frame cost on scenes real apps are made of,
// the ones a Fyne comparison reported as regressions (#427). Unit benchmarks
// cannot see these: the cost is in a full paint of a full window.
//
// Build and run from the repository root:
//
//	go run ./hack/perfscenes -scene spinner -n 100
//
// Scenes:
//   - shapes: n rectangles and circles (default 20000), all moved every frame
//   - labels: n labels in a scroll container (default 2000), all re-texted every frame
//   - tiles: n cards with a label, progress bar and button (default 500), all updated every frame
//   - spinner: the same n cards left static, plus one infinite progress bar
//
// The window is 1920×1080. One mutation is applied per painted frame: a 1×1
// raster refreshed with each mutation marks when a frame was painted. After a
// warm-up, one line is printed:
//
//	RESULT scene=... frames=... fps=... mean_ms=... p99_ms=... cpu_ms_per_frame=...
//	       alloc_kb_per_frame=... allocs_per_frame=... heap_mb=... maxrss_mb=... gc=...
//
// cpu_ms_per_frame counts every thread of the process, so it can exceed
// mean_ms. On a vsync'd display fps is capped at the refresh rate, which makes
// cpu_ms_per_frame the better cost figure for cheap scenes.
//
// The same program builds against Fyne by renaming the imports (bellui →
// fyne.io/fyne/v2), core.CanvasObject → fyne.CanvasObject and geo.NewSize/NewPos
// → fyne.NewSize/NewPos, and replacing Show + Run with ShowAndRun.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"os/exec"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"syscall"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

var (
	scene      = flag.String("scene", "spinner", "shapes, labels, tiles or spinner")
	count      = flag.Int("n", 0, "object count (0: the scene's default)")
	secs       = flag.Float64("secs", 8, "seconds measured")
	warm       = flag.Float64("warm", 2, "seconds of warm-up before measuring")
	memProfile = flag.String("memprofile", "", "write an allocation profile of the measured window to this file")
	cpuProfile = flag.String("cpuprofile", "", "write a CPU profile of the measured window to this file")
	maxGap     = flag.Duration("maxgap", 250*time.Millisecond, "refuse the result if any frame took longer than this")
)

// painted receives the time of each frame that drew the marker raster.
var painted = make(chan time.Time, 1)

func main() {
	flag.Parse()
	// A window that never paints (no display session, an occluded window)
	// can leave the app waiting forever, which ran a CI job out of time
	// (#439). Give up well after the run should have ended.
	limit := time.Duration((*warm+*secs)*float64(time.Second)) + 30*time.Second
	time.AfterFunc(limit, func() {
		fmt.Fprintf(os.Stderr, "perfscenes: no result after %v, giving up\n", limit)
		os.Exit(3)
	})
	a := app.NewWithID("com.bellui.perfscenes")
	w := a.NewWindow("perfscenes")
	mark := canvas.NewRaster(func(int, int) image.Image {
		select {
		case painted <- time.Now():
		default:
		}
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	})

	content, mutate, err := build(*scene, mark)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	w.SetContent(content)
	w.Resize(geo.NewSize(1920, 1080))
	// A window the OS considers hidden (covered, minimised, on another Space,
	// the display asleep) gets few or no frames, which measures the OS, not
	// the toolkit. Stay on top, keep the display awake, and refuse a run in
	// which frames stopped (maxgap).
	if dw, ok := w.(desktop.Window); ok {
		dw.RequestAlwaysOnTop()
	}
	keepDisplayAwake()
	go func() {
		if err := measure(mark, mutate); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		bellui.Do(a.Quit)
	}()
	w.Show()
	a.Run()
}

func objectCount(def int) int {
	if *count > 0 {
		return *count
	}
	return def
}

// build returns the scene's content and the change it applies each frame (nil
// for a scene that changes nothing itself).
func build(name string, mark *canvas.Raster) (core.CanvasObject, func(frame int), error) {
	switch name {
	case "shapes":
		n := objectCount(20000)
		objs := make([]core.CanvasObject, 0, n+1)
		for i := 0; i < n; i++ {
			c := color.NRGBA{R: uint8(i), G: uint8(i * 7), B: uint8(i * 13), A: 255}
			var o core.CanvasObject = canvas.NewRectangle(c)
			if i%2 == 1 {
				o = canvas.NewCircle(c)
			}
			o.Resize(geo.NewSize(12, 12))
			objs = append(objs, o)
		}
		mark.Resize(geo.NewSize(1, 1))
		content := container.NewWithoutLayout(append(objs, mark)...)
		return content, func(f int) {
			for i, o := range objs {
				t := float64(f+i) * 0.01
				o.Move(geo.NewPos(float32(900+800*math.Sin(t*1.3+float64(i))), float32(500+450*math.Cos(t+float64(i)*0.7))))
			}
			content.Refresh()
		}, nil

	case "labels":
		n := objectCount(2000)
		labels := make([]*widget.Label, n)
		objs := make([]core.CanvasObject, n)
		for i := range labels {
			labels[i] = widget.NewLabel("0")
			objs[i] = labels[i]
		}
		grid := container.NewVScroll(container.NewGridWrap(geo.NewSize(48, 20), objs...))
		return container.NewBorder(mark, nil, nil, nil, grid), func(f int) {
			for i, l := range labels {
				l.SetText(strconv.Itoa(f + i))
			}
		}, nil

	case "tiles", "spinner":
		n := objectCount(500)
		values := make([]*widget.Label, n)
		bars := make([]*widget.ProgressBar, n)
		objs := make([]core.CanvasObject, n)
		for i := range objs {
			values[i], bars[i] = widget.NewLabel("0"), widget.NewProgressBar()
			body := container.NewVBox(values[i], bars[i], widget.NewButton("Open", func() {}))
			objs[i] = widget.NewCard("Tile "+strconv.Itoa(i), "", body)
		}
		grid := container.NewVScroll(container.NewGridWrap(geo.NewSize(180, 200), objs...))
		if name == "spinner" {
			top := container.NewHBox(mark, widget.NewProgressBarInfinite())
			return container.NewBorder(top, nil, nil, nil, grid), nil, nil
		}
		return container.NewBorder(mark, nil, nil, nil, grid), func(f int) {
			for i := range values {
				v := (f + i) % 100
				values[i].SetText(strconv.Itoa(v))
				bars[i].SetValue(float64(v) / 100)
			}
		}, nil
	}
	return nil, nil, fmt.Errorf("unknown scene %q", name)
}

// measure applies one mutation per painted frame, warms up, then measures for
// -secs and prints the RESULT line.
func measure(mark *canvas.Raster, mutate func(int)) error {
	time.Sleep(500 * time.Millisecond)
	warmEnd := time.Now().Add(time.Duration(*warm * float64(time.Second)))
	var (
		frames         []time.Time
		before, after  runtime.MemStats
		cpuStart       time.Duration
		start          time.Time
		measuring      bool
		stopCPUProfile = func() {}
	)
	for f := 0; ; f++ {
		bellui.DoAndWait(func() {
			if mutate != nil {
				mutate(f)
			}
			mark.Refresh()
		})
		var at time.Time
		select {
		case at = <-painted:
		case <-time.After(2 * time.Second):
			return fmt.Errorf("no frame painted within 2s (frame %d)", f)
		}
		if !measuring && at.After(warmEnd) {
			measuring = true
			if *memProfile != "" {
				runtime.MemProfileRate = 4096
			}
			if *cpuProfile != "" {
				stop, err := startCPUProfile(*cpuProfile)
				if err != nil {
					return err
				}
				stopCPUProfile = stop
			}
			runtime.ReadMemStats(&before)
			cpuStart, start = cpuTime(), at
		}
		if measuring {
			frames = append(frames, at)
			if at.Sub(start).Seconds() >= *secs {
				break
			}
		}
	}
	runtime.ReadMemStats(&after)
	cpu := cpuTime() - cpuStart
	stopCPUProfile()
	if *memProfile != "" {
		if err := writeProfile("allocs", *memProfile); err != nil {
			return err
		}
	}

	n := len(frames) - 1
	if n < 1 {
		return fmt.Errorf("only %d frames measured", len(frames))
	}
	wall := frames[n].Sub(frames[0])
	gaps := make([]float64, n)
	for i := range gaps {
		gaps[i] = float64(frames[i+1].Sub(frames[i]).Microseconds()) / 1000
	}
	sort.Float64s(gaps)
	if worst := time.Duration(gaps[n-1] * float64(time.Millisecond)); worst > *maxGap {
		return fmt.Errorf("a frame took %v (over -maxgap %v): the window was probably hidden or the display asleep; no result", worst.Round(time.Millisecond), *maxGap)
	}
	p99 := gaps[int(math.Ceil(float64(n)*0.99))-1]
	nf := float64(n)
	fmt.Printf("RESULT scene=%s n=%d frames=%d fps=%.1f mean_ms=%.2f p99_ms=%.2f cpu_ms_per_frame=%.2f alloc_kb_per_frame=%.1f allocs_per_frame=%.0f heap_mb=%.1f maxrss_mb=%.0f gc=%d\n",
		*scene, *count, n, nf/wall.Seconds(), wall.Seconds()*1000/nf, p99,
		float64(cpu.Microseconds())/1000/nf,
		float64(after.TotalAlloc-before.TotalAlloc)/1024/nf, float64(after.Mallocs-before.Mallocs)/nf,
		float64(after.HeapInuse)/(1<<20), maxRSSMB(), after.NumGC-before.NumGC)
	return nil
}

func startCPUProfile(path string) (func(), error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		pprof.StopCPUProfile()
		f.Close()
	}, nil
}

func writeProfile(name, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := pprof.Lookup(name).WriteTo(f, 0); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// keepDisplayAwake keeps the display and the system awake while this process
// runs, on macOS (caffeinate exits when the process it watches does).
func keepDisplayAwake() {
	if runtime.GOOS != "darwin" {
		return
	}
	_ = exec.Command("caffeinate", "-d", "-i", "-w", strconv.Itoa(os.Getpid())).Start()
}

// cpuTime is the process's user plus system time, across all threads.
func cpuTime() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}

// maxRSSMB is the process's peak resident set size in MB. getrusage reports it
// in bytes on macOS and in kilobytes on Linux.
func maxRSSMB() float64 {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	if runtime.GOOS == "darwin" {
		return float64(ru.Maxrss) / (1 << 20)
	}
	return float64(ru.Maxrss) / 1024
}
