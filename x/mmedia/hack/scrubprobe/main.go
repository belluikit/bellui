// Command scrubprobe is the live-verification harness for the seek-bar scrub
// preview (#89).
//
// It is deliberately interactive-only, and that is the point. Everything about
// the preview that can be asserted is asserted in the unit tests: the keyframe
// decode against a real fixture, the request coalescing, the placement
// arithmetic, and — via a rendered capture — that the thumbnail is painted above
// the controls and over the video rather than hidden underneath them. What no
// test can answer is whether it *feels* right: whether the thumbnail keeps up
// with the pointer, whether the frame it shows is the frame you expected, and
// whether it gets in the way.
//
// A scripted mode was considered and rejected. Driving it would need synthetic
// hovers and a way to read the preview's state back, which means public API on
// MediaPlayer that exists only for this program — and an application has no use
// for either. The unit tests cover that ground already.
//
// It is a separate module (its own go.mod) so x/mmedia's library go.mod stays
// free of the GL/app dependency tree, matching hack/captureprobe.
//
//	cd hack/scrubprobe
//	go run .
//	go run . -media /path/to/something-long.mkv
//	go run . -media https://example.com/video.mp4
//
// What to look for, hovering along the seek bar without pressing anything:
//
//   - a thumbnail appears above the bar, over the video, and follows the pointer
//   - the frame it shows is roughly the frame at that position — approximate is
//     correct, since the preview stops at the keyframe rather than decoding
//     forward to the exact target
//   - the timestamp under it tracks the pointer exactly, even while the picture
//     lags a decode behind
//   - playback does not move, and the thumb does not jump to the pointer
//   - near either end the thumbnail stays fully on screen
//   - it disappears when the pointer leaves the bar
//
// Worth trying on something long with a large GOP, not just the 4-second test
// fixture: that is where the difference between a keyframe fetch and a decode
// forward to the exact position is visible rather than theoretical.
package main

import (
	"flag"
	"fmt"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/mmedia"
)

func main() {
	media := flag.String("media", "../../internal/source/testdata/sample.mp4",
		"path or url to play")
	flag.Parse()

	a := app.New()
	w := a.NewWindow("scrubprobe — hover the seek bar")

	p, err := mmedia.NewMediaPlayerURL(*media, mmedia.WithScrubPreview(), mmedia.WithAutoPlay())
	if err != nil {
		fmt.Printf("could not open %q with a scrub preview: %v\n", *media, err)
		w.SetContent(container.NewVBox(
			widget.NewLabel("could not open the media with a scrub preview:"),
			widget.NewLabel(err.Error()),
		))
		w.Resize(geo.NewSize(560, 160))
		bellui.ShowAndRun(w)
		return
	}
	defer p.Close()

	w.SetContent(container.NewBorder(
		widget.NewLabel("Hover along the seek bar — do not press. A thumbnail should "+
			"follow the pointer, and playback should not move."),
		nil, nil, nil, p,
	))
	w.Resize(geo.NewSize(760, 640))
	bellui.ShowAndRun(w)
}
