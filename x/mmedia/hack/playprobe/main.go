// Command playprobe is the live-verify probe for actual video playback
// through the streaming raster path (#100) — decode → WriteFrame → painter,
// the whole pipeline a VideoPlayer drives, where hack/rasterstream covers only
// the canvas.Raster layer beneath it.
//
//	go run . <file-or-url>          # default painter (WebGPU on desktop)
//	go run -tags gl . <file-or-url> # GL painter
//
// There is no bundled clip; make one with FFmpeg's synthetic test source —
// its constant motion is what the -auto check needs:
//
//	ffmpeg -f lavfi -i testsrc=duration=5:size=320x240:rate=30 /tmp/testsrc.mp4
//
// PASS: the clip visibly plays (manual), or -auto reports changed pixels
// inside the player's bounds between two captures a second apart.
// FAIL: a still first frame — the exact symptom of a streaming texture the
// painter never updates in place (#34's freeze, which #100 re-enables the
// guard against believing is still there).
//
//	go run . -auto /tmp/testsrc.mp4
package main

import (
	"flag"
	"fmt"
	"image"
	"os"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/core"

	"github.com/belluikit/bellui/x/mmedia"
)

func main() {
	auto := flag.Bool("auto", false, "capture twice and fail on a frozen frame instead of needing a human")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: playprobe [-auto] <file-or-url>")
		os.Exit(2)
	}
	src := flag.Arg(0)

	a := app.New()
	w := a.NewWindow("playprobe (#100)")

	player, err := mmedia.NewVideoPlayerURL(src, mmedia.WithAutoPlay(), mmedia.WithLoop())
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: open %s: %v\n", src, err)
		os.Exit(1)
	}
	w.SetContent(player)

	if *auto {
		go runAuto(a, w, player)
	}
	bellui.ShowAndRun(w)
}

// runAuto captures the live canvas twice and counts changed pixels inside the
// player's bounds. Runs off the main goroutine; every canvas touch hops back
// via bellui.Do/DoAndWait (the hack/rasterstream pattern).
func runAuto(a bellui.App, w bellui.Window, player core.CanvasObject) {
	capture := func() image.Image {
		var img image.Image
		bellui.DoAndWait(func() { img = w.Canvas().Capture() })
		return img
	}

	time.Sleep(2 * time.Second) // window mapped, playback started
	first := capture()
	time.Sleep(time.Second)
	second := capture()

	exit := func(code int) {
		bellui.Do(func() { a.Quit() })
		time.Sleep(200 * time.Millisecond)
		os.Exit(code)
	}

	if first == nil || second == nil {
		fmt.Println("FAIL: canvas capture unavailable")
		exit(1)
	}

	var r image.Rectangle
	bellui.DoAndWait(func() {
		pos := bellui.CurrentApp().Driver().AbsolutePositionForObject(player)
		size := player.Size()
		s := w.Canvas().Scale()
		r = image.Rect(
			int(pos.X*s), int(pos.Y*s),
			int((pos.X+size.Width)*s), int((pos.Y+size.Height)*s),
		)
	})

	changed := 0
	r = r.Intersect(first.Bounds()).Intersect(second.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			ar, ag, ab, aa := first.At(x, y).RGBA()
			br, bg, bb, ba := second.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				changed++
			}
		}
	}

	if changed == 0 {
		fmt.Println("FAIL: playback is frozen — 0 pixels changed inside the player between captures")
		exit(1)
	}
	fmt.Printf("PASS: playback is live — %d pixels changed inside the player between captures\n", changed)
	exit(0)
}
