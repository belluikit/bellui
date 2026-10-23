package mmedia

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"io"
	"os"
	"testing"
	"time"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/x/mmedia/internal/pipeline"
	"github.com/belluikit/bellui/x/mmedia/internal/source"
)

// hoverSeekBar drives a real hover across the seek bar rather than calling the
// callback directly, so the wiring from the slider's OnHover through to
// OnScrub is exercised the way the driver drives it.
func hoverSeekBar(tc *TransportControls, x float32) {
	tc.seekBar.Resize(geo.NewSize(200, tc.seekBar.MinSize().Height))
	tc.seekBar.HoverMoved(&event.HoverEvent{
		PointEvent: event.PointEvent{Position: geo.NewPos(x, 0)},
	})
}

// TestSeekBarHoverReportsAScrubPosition is the gesture #89 is built on: hovering
// reports where the pointer is without seeking there. #44 added the slider
// callbacks; this is the transport using them.
func TestSeekBarHoverReportsAScrubPosition(t *testing.T) {
	tc := NewTransportControls()
	var scrubs []time.Duration
	var seeks []time.Duration
	tc.OnScrub = func(at time.Duration) { scrubs = append(scrubs, at) }
	tc.OnSeek = func(at time.Duration) { seeks = append(seeks, at) }
	tc.SetTime(0, time.Minute)

	hoverSeekBar(tc, 1000) // past the right-hand end: clamps to 1.0

	if len(scrubs) != 1 {
		t.Fatalf("hover reported %d scrub(s), want 1: %v", len(scrubs), scrubs)
	}
	if scrubs[0] != time.Minute {
		t.Errorf("scrub position = %v, want the end of a one-minute source", scrubs[0])
	}
	// The whole point: looking is not committing.
	if len(seeks) != 0 {
		t.Errorf("hovering issued %d seek(s), want none: %v", len(seeks), seeks)
	}
}

func TestSeekBarHoverEndReportsScrubEnd(t *testing.T) {
	tc := NewTransportControls()
	ended := 0
	tc.OnScrubEnd = func() { ended++ }
	tc.SetTime(0, time.Minute)

	hoverSeekBar(tc, 100)
	tc.seekBar.HoverOut()

	if ended != 1 {
		t.Errorf("HoverOut reported %d scrub end(s), want 1", ended)
	}
}

// TestSeekBarHoverIsInertForALiveSource: a camera has no timeline, the bar is
// disabled for that reason, and there is no position under the pointer to
// preview. Reporting one would have the player decode against a source that
// cannot seek.
func TestSeekBarHoverIsInertForALiveSource(t *testing.T) {
	tc := NewTransportControls()
	scrubs := 0
	tc.OnScrub = func(time.Duration) { scrubs++ }
	tc.SetSeekable(false)
	tc.SetTime(30*time.Second, 0)

	hoverSeekBar(tc, 100)

	if scrubs != 0 {
		t.Errorf("hovering a live source reported %d scrub(s), want none", scrubs)
	}
}

// TestScrubPreviewShowsAndHidesWithTheHover covers the visibility handover: the
// preview appears on the first hover and goes away when the pointer leaves.
func TestScrubPreviewShowsAndHidesWithTheHover(t *testing.T) {
	tc := NewTransportControls()
	tc.SetTime(0, time.Minute)

	p := newScrubPreview(urlTestMedia)
	defer p.close()
	tc.SetScrubPreview(p.object(), p.size())

	if p.object().Visible() {
		t.Fatal("the preview is visible before anything was hovered")
	}

	hoverSeekBar(tc, 100)
	if !p.object().Visible() {
		t.Error("the preview did not appear on hover")
	}

	tc.seekBar.HoverOut()
	if p.object().Visible() {
		t.Error("the preview did not go away when the pointer left")
	}
}

// TestScrubPreviewLayoutTracksThePointer checks the placement: the thumbnail is
// centred on the hovered position and sits above the controls, not inside them.
func TestScrubPreviewLayoutTracksThePointer(t *testing.T) {
	tc := NewTransportControls()
	tc.SetTime(0, time.Minute)

	p := newScrubPreview(urlTestMedia)
	defer p.close()
	tc.SetScrubPreview(p.object(), p.size())

	// Place the seek bar somewhere non-trivial, so a layout that forgot to add
	// the bar's own offset would land in the wrong place. The seek bar is nested
	// *inside* the bar container, which is why the container passed as the first
	// object below is a separate one — passing the slider itself would have the
	// layout resize it and destroy the geometry under test.
	tc.seekBar.Resize(geo.NewSize(200, 20))
	tc.seekBar.Move(geo.NewPos(120, 14))
	tc.previewShown = true
	tc.previewAt = 0.5

	l := &scrubPreviewLayout{controls: tc}
	bar := canvas.NewRectangle(color.Transparent)
	l.Layout([]core.CanvasObject{bar, p.object()}, geo.NewSize(600, 48))

	pos := p.object().Position()
	wantX := 120 + 100 - p.size().Width/2 // bar offset + half the track, centred
	if pos.X != wantX {
		t.Errorf("preview x = %v, want %v (centred on the hovered position)", pos.X, wantX)
	}
	if pos.Y >= 0 {
		t.Errorf("preview y = %v, want it above the controls (negative)", pos.Y)
	}
	if got := p.object().Size(); got != p.size() {
		t.Errorf("preview size = %v, want %v", got, p.size())
	}
}

// TestScrubPreviewLayoutKeepsThePreviewOnScreen: near either end, being exactly
// over the thumb would put half the thumbnail outside the window.
func TestScrubPreviewLayoutKeepsThePreviewOnScreen(t *testing.T) {
	tc := NewTransportControls()
	tc.SetTime(0, time.Minute)
	p := newScrubPreview(urlTestMedia)
	defer p.close()
	tc.SetScrubPreview(p.object(), p.size())

	tc.seekBar.Resize(geo.NewSize(600, 20))
	tc.seekBar.Move(geo.NewPos(0, 14))
	tc.previewShown = true

	l := &scrubPreviewLayout{controls: tc}
	bar := canvas.NewRectangle(color.Transparent)
	size := geo.NewSize(600, 48)

	tc.previewAt = 0
	l.Layout([]core.CanvasObject{bar, p.object()}, size)
	if got := p.object().Position().X; got < 0 {
		t.Errorf("preview x = %v at the start of the bar, want it clamped to 0", got)
	}

	tc.previewAt = 1
	l.Layout([]core.CanvasObject{bar, p.object()}, size)
	if got, max := p.object().Position().X, size.Width-p.size().Width; got > max {
		t.Errorf("preview x = %v at the end of the bar, want it clamped to %v", got, max)
	}
}

// TestScrubPreviewPaintsOverTheVideo is the load-bearing visual claim, and the
// riskiest assumption in the whole feature: the preview is placed at a negative
// Y inside the controls, so it is drawn outside their bounds and over the video.
// That relies on a bellui.Container not clipping its children, and on the player
// ordering the controls after the video so they paint on top.
//
// Asserted by rendering rather than by reasoning. A regression — a clip, or the
// old [controls, video] order — leaves the preview invisible, and no positional
// assertion would notice.
func TestScrubPreviewPaintsOverTheVideo(t *testing.T) {
	p, err := NewMediaPlayerURL(urlTestMedia, WithScrubPreview())
	if err != nil {
		t.Fatalf("NewMediaPlayerURL: %v", err)
	}
	defer p.Close()

	w := test.NewTempWindow(t, p)
	w.Resize(geo.NewSize(640, 480))

	// A frame the preview cannot be confused with: solid magenta, which neither
	// the theme nor the test fixture produces.
	marker := image.NewRGBA(image.Rect(0, 0, 32, 24))
	for i := 0; i < len(marker.Pix); i += 4 {
		marker.Pix[i], marker.Pix[i+1], marker.Pix[i+2], marker.Pix[i+3] = 0xFF, 0x00, 0xFF, 0xFF
	}

	// The shown state is set directly rather than by driving a hover. A hover
	// would also kick off a real decode, whose goroutine writes the same image
	// this test is writing — a race, and one that says nothing about painting.
	// The hover-to-shown wiring has its own test; this one is only about whether
	// what is shown ends up on screen.
	showPreview(p, marker, 0.5)

	img := w.Canvas().Capture()
	if !containsColor(img, 0xFF, 0x00, 0xFF) {
		t.Error("the preview was not painted — a negative-Y child of the controls is " +
			"being clipped, or the video is painting over the controls")
	}

	// And it must go away again, or it would sit over the video permanently.
	p.controls.previewShown = false
	p.preview.object().Hide()
	p.controls.Refresh()
	w.Resize(geo.NewSize(640, 480))
	if containsColor(w.Canvas().Capture(), 0xFF, 0x00, 0xFF) {
		t.Error("the preview is still painted after the pointer left the seek bar")
	}
}

// showPreview puts the preview on screen with a known frame, without going
// through a hover and the decode it would start.
func showPreview(p *MediaPlayer, frame image.Image, at float64) {
	p.preview.img.Image = frame
	p.preview.img.Refresh()
	p.controls.previewAt = at
	p.controls.previewShown = true
	p.preview.object().Show()
	p.controls.Refresh()
	// A resize is what actually re-runs the layout that places the preview.
	p.Resize(p.Size())
}

// containsColor reports whether img has a pixel of roughly this colour. Rough
// because the painter composites and scales, so an exact match is not something
// to depend on.
func containsColor(img image.Image, wantR, wantG, wantB uint8) bool {
	const tolerance = 24
	near := func(got uint32, want uint8) bool {
		g := int(got >> 8)
		d := g - int(want)
		return d <= tolerance && d >= -tolerance
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if near(r, wantR) && near(g, wantG) && near(bl, wantB) {
				return true
			}
		}
	}
	return false
}

// TestControlsKnowTheDurationBeforePlayback is a bug the rendering test above
// exposed, and it predates the preview: TransportControls learns the duration
// from the first playback time tick, so before anything had played tc.duration
// was zero — and both the seek on release and the scrub preview convert a
// fraction through it and do nothing while it is.
//
// Scrubbing a file that has not been started is precisely what a preview is for,
// so the player now tells the transport the duration at construction.
func TestControlsKnowTheDurationBeforePlayback(t *testing.T) {
	p, err := NewMediaPlayerURL(urlTestMedia)
	if err != nil {
		t.Fatalf("NewMediaPlayerURL: %v", err)
	}
	defer p.Close()

	if p.controls.duration <= 0 {
		t.Fatalf("controls duration = %v before playback, want the media's %v",
			p.controls.duration, p.Duration())
	}

	// The behaviour that depended on it: a seek gesture before anything played.
	var seeks []time.Duration
	p.controls.OnSeek = func(at time.Duration) { seeks = append(seeks, at) }
	p.controls.seekBar.Resize(geo.NewSize(200, p.controls.seekBar.MinSize().Height))
	p.controls.seekBar.Tapped(&event.PointEvent{Position: geo.NewPos(1000, 0)})

	if len(seeks) != 1 {
		t.Errorf("tapping the seek bar before playback issued %d seek(s), want 1", len(seeks))
	}
}

// TestScrubPreviewCoalescesRequests is the throttling the issue asks for: decode
// cost is per pointer-move, so a sweep across the bar must not queue one decode
// per pixel. Only the newest pending position survives.
func TestScrubPreviewCoalescesRequests(t *testing.T) {
	p := newScrubPreview(urlTestMedia)
	defer p.close()

	// Hold the worker off by claiming the in-flight slot, then pile requests up.
	p.mu.Lock()
	p.running = true
	p.mu.Unlock()

	for i := range 50 {
		p.request(time.Duration(i) * time.Second)
	}

	p.mu.Lock()
	pending, set := p.pending, p.pendingSet
	p.mu.Unlock()

	if !set {
		t.Fatal("no position pending after 50 requests")
	}
	if want := 49 * time.Second; pending != want {
		t.Errorf("pending = %v, want %v — only the newest request should survive", pending, want)
	}
}

// TestScrubPreviewDropsRequestsAfterClose guards the shutdown path: a request
// arriving after Close must not reopen the media behind the player's back.
func TestScrubPreviewDropsRequestsAfterClose(t *testing.T) {
	p := newScrubPreview(urlTestMedia)
	p.close()

	p.request(time.Second)

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pendingSet {
		t.Error("a request after close was accepted")
	}
	if p.running {
		t.Error("a decode was started after close")
	}
}

// TestWithScrubPreviewOnAURLPlayer is the supported path.
func TestWithScrubPreviewOnAURLPlayer(t *testing.T) {
	p, err := NewMediaPlayerURL(urlTestMedia, WithScrubPreview())
	if err != nil {
		t.Fatalf("NewMediaPlayerURL with a scrub preview: %v", err)
	}
	defer p.Close()

	if p.preview == nil {
		t.Fatal("no preview was installed")
	}
	if p.controls.preview == nil {
		t.Error("the controls were not given the preview to place")
	}
}

// TestWithScrubPreviewOnAFile covers the *os.File case, where the name is what
// the second decoder reopens.
func TestWithScrubPreviewOnAFile(t *testing.T) {
	f, err := os.Open(urlTestMedia)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	p, err := NewMediaPlayer(f, WithScrubPreview())
	if err != nil {
		t.Fatalf("NewMediaPlayer on a file with a scrub preview: %v", err)
	}
	defer p.Close()
	if p.preview == nil {
		t.Error("no preview was installed for a file-backed player")
	}
}

// TestWithScrubPreviewRejectsAPlainReader is the fork's rule that a failed
// capability degrades visibly. There is no second reader to give the preview
// decoder, and silently giving no preview would leave the caller to discover it
// by hovering.
func TestWithScrubPreviewRejectsAPlainReader(t *testing.T) {
	data, err := os.ReadFile(urlTestMedia)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	p, err := NewMediaPlayer(struct{ io.ReadSeeker }{bytes.NewReader(data)}, WithScrubPreview())
	if err == nil {
		p.Close()
		t.Fatal("a reader-backed player accepted WithScrubPreview")
	}
	if !errors.Is(err, ErrNoScrubPreviewSource) {
		t.Errorf("error = %v, want ErrNoScrubPreviewSource", err)
	}
}

// liveStubSource stands in for a camera so the live path can be built with no
// device present. It claims source.LiveSource, which is what the pipeline keys
// its live behaviour off.
type liveStubSource struct{}

func (liveStubSource) HasVideo() bool                        { return true }
func (liveStubSource) HasAudio() bool                        { return false }
func (liveStubSource) VideoSize() (int, int)                 { return 16, 16 }
func (liveStubSource) Duration() time.Duration               { return 0 }
func (liveStubSource) Framerate() float64                    { return 30 }
func (liveStubSource) SampleRate() int                       { return 0 }
func (liveStubSource) Channels() int                         { return 0 }
func (liveStubSource) SetVideoCallback(source.VideoCallback) {}
func (liveStubSource) SetAudioCallback(source.AudioCallback) {}
func (liveStubSource) SetAudioChannels(int) error            { return nil }
func (liveStubSource) SetAudioSampleRate(int) error          { return nil }
func (liveStubSource) Decode(time.Duration)                  {}
func (liveStubSource) Seek(time.Duration)                    {}
func (liveStubSource) Rewind()                               {}
func (liveStubSource) HasEnded() bool                        { return false }
func (liveStubSource) Close()                                {}
func (liveStubSource) IsLive() bool                          { return true }

// TestWithScrubPreviewIsInertForALiveSource: a player written to show either a
// file or a camera should be able to pass the option unconditionally, so a live
// source gets no preview and no complaint. A camera has no timeline to preview
// along and its seek bar is disabled for the same reason.
//
// Note this is the one case where a "" preview url does *not* produce
// ErrNoScrubPreviewSource, so the ordering of the two checks is what is being
// pinned here.
func TestWithScrubPreviewIsInertForALiveSource(t *testing.T) {
	pipe := pipeline.NewPipelineForSource(liveStubSource{}, pipeline.Config{})
	mp, err := newMediaPlayerFromPipeline(pipe, buildOptions([]Option{WithScrubPreview()}), "")
	if err != nil {
		t.Fatalf("a live player must accept WithScrubPreview without complaint: %v", err)
	}
	defer mp.Close()

	if mp.preview != nil {
		t.Error("a live source was given a scrub preview; there is no timeline to preview along")
	}
	if mp.controls.preview != nil {
		t.Error("the controls were given a preview to place for a live source")
	}
}
