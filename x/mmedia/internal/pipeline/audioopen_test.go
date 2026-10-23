package pipeline

import (
	"errors"
	"testing"
	"time"

	"github.com/belluikit/bellui/x/mmedia/internal/audio"
	"github.com/belluikit/bellui/x/mmedia/internal/source"
)

// newTestPipeline builds a pipeline around a test source, failing the test if
// it cannot be built.
func newTestPipeline(t *testing.T, src source.Source) *Pipeline {
	t.Helper()
	p, err := newPipelineForSource(src, Config{})
	if err != nil {
		t.Fatalf("newPipelineForSource: %v", err)
	}
	return p
}

// withAudioOutput replaces the audio device for one test.
func withAudioOutput(t *testing.T, open func(channels int) (*audio.Output, error)) {
	t.Helper()
	saved := newAudioOutput
	newAudioOutput = open
	t.Cleanup(func() { newAudioOutput = saved })
}

// detachedDevice is a device that opens, at the channel count asked for, and
// plays nothing, so a pipeline with audio can be built with no sound card.
func detachedDevice(channels int) (*audio.Output, error) {
	return audio.NewDetachedOutput(channels), nil
}

// audioSource is a stream with audio at its own format, whose conversion to
// the device's format can be made to fail.
type audioSource struct {
	*shapedSource
	channelsErr, rateErr     error
	askedChannels, askedRate int
}

func newAudioSource(rate, channels int) *audioSource {
	s := newShapedSource(16, 16)
	s.audio, s.sampleRate, s.channels = true, rate, channels
	return &audioSource{shapedSource: s}
}

func (s *audioSource) SetAudioChannels(n int) error {
	s.askedChannels = n
	return s.channelsErr
}

func (s *audioSource) SetAudioSampleRate(rate int) error {
	s.askedRate = rate
	return s.rateErr
}

// buildWithin runs build and fails the test, rather than hanging it, if it
// does not return: a device that would not open once blocked the first player
// forever (#690).
func buildWithin(t *testing.T, build func() (*Pipeline, error)) (*Pipeline, error) {
	t.Helper()
	type result struct {
		p   *Pipeline
		err error
	}
	done := make(chan result, 1)
	go func() {
		p, err := build()
		done <- result{p, err}
	}()
	select {
	case r := <-done:
		return r.p, r.err
	case <-time.After(5 * time.Second):
		t.Fatal("building the pipeline did not return")
		return nil, nil
	}
}

// TestConstructionFailsWhenTheDeviceWillNotOpen: the owner's decision on #690
// is that a device that will not open fails the player's construction. The
// source was the caller's and stays open for it to close.
func TestConstructionFailsWhenTheDeviceWillNotOpen(t *testing.T) {
	refused := errors.New("no audio device")
	withAudioOutput(t, func(int) (*audio.Output, error) { return nil, refused })

	src := newAudioSource(44100, 2)
	p, err := buildWithin(t, func() (*Pipeline, error) { return NewPipelineForSource(src) })
	if !errors.Is(err, refused) {
		t.Fatalf("NewPipelineForSource error = %v, want it to wrap %v", err, refused)
	}
	if p != nil {
		t.Error("a pipeline was returned along with the error")
	}
	if src.closed {
		t.Error("NewPipelineForSource closed a source its caller still owns")
	}
}

// TestConstructionFailsWhenTheRateCannotBeConverted pins #698: the stream's
// rate conversion to the device's used to fail into an OnError that was always
// nil at construction, and a 44.1 kHz stream then played 8.8% fast.
func TestConstructionFailsWhenTheRateCannotBeConverted(t *testing.T) {
	withAudioOutput(t, detachedDevice)
	src := newAudioSource(44100, 2)
	src.rateErr = errors.New("swr_init failed")

	_, err := buildWithin(t, func() (*Pipeline, error) { return NewPipelineForSource(src) })
	if !errors.Is(err, src.rateErr) {
		t.Fatalf("NewPipelineForSource error = %v, want it to wrap %v", err, src.rateErr)
	}
	if src.askedRate != audio.DeviceSampleRate {
		t.Errorf("the source was asked for %d Hz, want %d", src.askedRate, audio.DeviceSampleRate)
	}
}

// TestConstructionFailsWhenTheChannelsCannotBeConverted is #698's other half,
// which predates the 48 kHz device: a mono stream on the stereo device.
func TestConstructionFailsWhenTheChannelsCannotBeConverted(t *testing.T) {
	withAudioOutput(t, detachedDevice)
	src := newAudioSource(audio.DeviceSampleRate, 1)
	src.channelsErr = errors.New("no such layout")

	_, err := buildWithin(t, func() (*Pipeline, error) { return NewPipelineForSource(src) })
	if !errors.Is(err, src.channelsErr) {
		t.Fatalf("NewPipelineForSource error = %v, want it to wrap %v", err, src.channelsErr)
	}
	if src.askedChannels != 2 {
		t.Errorf("the source was asked for %d channels, want 2", src.askedChannels)
	}
}

// TestConstructionConvertsToTheDevice is the positive control for the two
// above: the same source with working conversion builds, and was asked for the
// device's format.
func TestConstructionConvertsToTheDevice(t *testing.T) {
	withAudioOutput(t, detachedDevice)
	src := newAudioSource(44100, 1)

	p, err := buildWithin(t, func() (*Pipeline, error) { return NewPipelineForSource(src) })
	if err != nil {
		t.Fatalf("NewPipelineForSource: %v", err)
	}
	defer p.Close()
	if src.askedRate != audio.DeviceSampleRate || src.askedChannels != 2 {
		t.Errorf("asked for %d Hz × %d, want %d Hz × 2", src.askedRate, src.askedChannels, audio.DeviceSampleRate)
	}
}

// TestAnOwnedSourceIsClosedWhenConstructionFails: NewPipeline and
// NewPipelineURL open the source themselves, so when the pipeline cannot be
// built nobody else can close it.
func TestAnOwnedSourceIsClosedWhenConstructionFails(t *testing.T) {
	withAudioOutput(t, detachedDevice)
	src := newAudioSource(44100, 2)
	src.rateErr = errors.New("swr_init failed")

	if _, err := newOwnedPipeline(src, Config{}); !errors.Is(err, src.rateErr) {
		t.Fatalf("newOwnedPipeline error = %v, want it to wrap %v", err, src.rateErr)
	}
	if !src.closed {
		t.Error("the source this package opened was left open")
	}
}

// TestSwapSourceReportsAFailedConversion: after construction there is no
// constructor to fail, and the new source is already adopted, so a conversion
// failure on a swap goes to OnError, as it always did.
func TestSwapSourceReportsAFailedConversion(t *testing.T) {
	withAudioOutput(t, detachedDevice)
	p := newTestPipeline(t, newAudioSource(audio.DeviceSampleRate, 2))
	defer p.Close()
	var reported error
	p.SetOnError(func(err error) { reported = err })

	fresh := newAudioSource(44100, 2)
	fresh.rateErr = errors.New("swr_init failed")
	if err := p.SwapSource(fresh); err != nil {
		t.Fatalf("SwapSource: %v", err)
	}
	if !errors.Is(reported, fresh.rateErr) {
		t.Errorf("OnError got %v, want it to wrap %v", reported, fresh.rateErr)
	}
}
