package pipeline

import (
	"errors"
	"image"
	"sync/atomic"
	"testing"
	"time"

	"github.com/belluikit/bellui/x/mmedia/internal/source"
)

// shapedSource is a stubSource whose media shape and geometry can be varied, so
// SwapSource can be driven through every branch with no device present.
type shapedSource struct {
	stubSource
	video, audio  bool
	width, height int
	channels      int
	sampleRate    int
	// decodes counts calls from the decode goroutine, so a test can see that a
	// source is actually being driven without reading stubSource's fields from
	// under it.
	decodes atomic.Int64
}

func newShapedSource(w, h int) *shapedSource {
	s := &shapedSource{video: true, width: w, height: h}
	s.frame = image.NewRGBA(image.Rect(0, 0, w, h))
	return s
}

func (s *shapedSource) HasVideo() bool        { return s.video }
func (s *shapedSource) HasAudio() bool        { return s.audio }
func (s *shapedSource) VideoSize() (int, int) { return s.width, s.height }
func (s *shapedSource) SampleRate() int       { return s.sampleRate }
func (s *shapedSource) Channels() int         { return s.channels }

func (s *shapedSource) Decode(elapsed time.Duration) {
	s.stubSource.Decode(elapsed)
	s.decodes.Add(1)
}

// liveShapedSource additionally claims source.LiveSource, so the live flag can
// be seen to follow the source across a swap.
type liveShapedSource struct{ *shapedSource }

func (liveShapedSource) IsLive() bool { return true }

var _ source.LiveSource = liveShapedSource{}

// TestSwapSourceReplacesAndClosesTheOld is the core of #96: the pipeline keeps
// running and the source underneath it is exchanged.
func TestSwapSourceReplacesAndClosesTheOld(t *testing.T) {
	old := newShapedSource(16, 16)
	p := newPipelineForSource(old, Config{})
	p.SetOnFrame(func(*image.RGBA) {})

	p.Play()
	time.Sleep(100 * time.Millisecond)

	fresh := newShapedSource(16, 16)
	if err := p.SwapSource(fresh); err != nil {
		t.Fatalf("SwapSource: %v", err)
	}
	defer p.Close()

	if !old.closed {
		t.Error("the replaced source must be closed — nothing else will ever release it")
	}
	if p.src != source.Source(fresh) {
		t.Error("the pipeline is not decoding from the new source")
	}
	if p.State() != StatePlaying {
		t.Errorf("state = %v, want playback to have resumed", p.State())
	}

	// The new source must actually be driven, not merely stored.
	time.Sleep(150 * time.Millisecond)
	if fresh.decodes.Load() == 0 {
		t.Error("the new source was never decoded from")
	}
}

// TestSwapSourceResizesTheFramePool covers the geometry change a different
// camera brings. The pool frames are allocated at the source's resolution and
// the video callback copies into them, so a larger frame would be truncated
// against buffers sized for the old device.
func TestSwapSourceResizesTheFramePool(t *testing.T) {
	p := newPipelineForSource(newShapedSource(16, 16), Config{})
	before := <-p.frameRecycle
	p.RecycleFrame(before)
	if got := before.Bounds().Dx(); got != 16 {
		t.Fatalf("precondition: pool frame width = %d, want 16", got)
	}

	if err := p.SwapSource(newShapedSource(64, 48)); err != nil {
		t.Fatalf("SwapSource: %v", err)
	}
	defer p.Close()

	if w, h := p.VideoSize(); w != 64 || h != 48 {
		t.Errorf("VideoSize = %dx%d, want 64x48", w, h)
	}
	after := <-p.frameRecycle
	if got := after.Bounds(); got.Dx() != 64 || got.Dy() != 48 {
		t.Errorf("pool frame = %v, want 64x48 — a larger frame would be truncated", got)
	}
}

// TestSwapSourceKeepsThePoolWhenGeometryIsUnchanged is the other half: swapping
// between two cameras of the same resolution must not drop and reallocate the
// whole pool.
func TestSwapSourceKeepsThePoolWhenGeometryIsUnchanged(t *testing.T) {
	p := newPipelineForSource(newShapedSource(16, 16), Config{})
	pool := p.frameRecycle

	if err := p.SwapSource(newShapedSource(16, 16)); err != nil {
		t.Fatalf("SwapSource: %v", err)
	}
	defer p.Close()

	if p.frameRecycle != pool {
		t.Error("the frame pool was rebuilt for a swap that did not change the geometry")
	}
}

// TestSwapSourceRefusesADifferentMediaShape guards the widget above the
// pipeline: it was built around HasVideo/HasAudio and cannot be rebuilt from
// down here, so a source that would change them is refused rather than adopted.
func TestSwapSourceRefusesADifferentMediaShape(t *testing.T) {
	old := newShapedSource(16, 16)
	p := newPipelineForSource(old, Config{})
	p.SetOnFrame(func(*image.RGBA) {})
	p.Play()
	defer p.Close()

	audioOnly := newShapedSource(16, 16)
	audioOnly.video, audioOnly.audio = false, true

	err := p.SwapSource(audioOnly)
	if !errors.Is(err, ErrSwapMedia) {
		t.Fatalf("SwapSource error = %v, want ErrSwapMedia", err)
	}
	// A refusal must be invisible, not merely survivable: nothing was torn down.
	if p.State() != StatePlaying {
		t.Errorf("state = %v, want playback undisturbed by a refused swap", p.State())
	}
	if p.src != source.Source(old) {
		t.Error("the original source was replaced despite the refusal")
	}
	if old.closed {
		t.Error("the original source was closed despite the refusal")
	}
	// The rejected source is still the caller's to close.
	if audioOnly.closed {
		t.Error("a refused source must not be closed by the pipeline")
	}
}

func TestSwapSourceRefusesNil(t *testing.T) {
	p := newPipelineForSource(newShapedSource(16, 16), Config{})
	defer p.Close()

	if err := p.SwapSource(nil); !errors.Is(err, ErrSwapMedia) {
		t.Errorf("SwapSource(nil) = %v, want ErrSwapMedia", err)
	}
}

// TestSwapSourceLeavesAStoppedPipelineStopped checks the state handover in the
// direction that is easy to get wrong: a swap is not a Play.
func TestSwapSourceLeavesAStoppedPipelineStopped(t *testing.T) {
	p := newPipelineForSource(newShapedSource(16, 16), Config{})
	p.SetOnFrame(func(*image.RGBA) {})
	defer p.Close()

	if err := p.SwapSource(newShapedSource(16, 16)); err != nil {
		t.Fatalf("SwapSource: %v", err)
	}
	if p.State() == StatePlaying {
		t.Error("swapping a source must not start playback that was not running")
	}
}

// TestSwapSourceTakesTheLiveFlagFromTheNewSource matters because live is what
// disables seeking and turns end-of-stream into a lost device. Carrying the old
// source's answer across would leave a file pretending to be a camera.
func TestSwapSourceTakesTheLiveFlagFromTheNewSource(t *testing.T) {
	p := newPipelineForSource(liveShapedSource{newShapedSource(16, 16)}, Config{})
	defer p.Close()
	if !p.IsLive() {
		t.Fatal("precondition: the pipeline should have started live")
	}

	if err := p.SwapSource(newShapedSource(16, 16)); err != nil {
		t.Fatalf("SwapSource: %v", err)
	}
	if p.IsLive() {
		t.Error("IsLive still reports the replaced source")
	}
}

// TestSwapSourceRebasesTheClock: a capture device stamps frames with absolute
// host time and FFmpegSource rebases on its first frame, which only works
// against a clock that starts where it expects.
func TestSwapSourceRebasesTheClock(t *testing.T) {
	p := newPipelineForSource(newShapedSource(16, 16), Config{})
	p.SetOnFrame(func(*image.RGBA) {})
	p.Play()
	time.Sleep(120 * time.Millisecond)
	if p.CurrentTime() == 0 {
		t.Fatal("precondition: the clock should have advanced")
	}

	if err := p.SwapSource(newShapedSource(16, 16)); err != nil {
		t.Fatalf("SwapSource: %v", err)
	}
	defer p.Close()

	if got := p.CurrentTime(); got > 50*time.Millisecond {
		t.Errorf("CurrentTime = %v just after a swap, want the clock restarted", got)
	}
}
