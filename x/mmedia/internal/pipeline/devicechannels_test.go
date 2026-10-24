package pipeline

import (
	"errors"
	"testing"

	"github.com/belluikit/bellui/x/mmedia/internal/audio"
)

// resetDeviceChannels gives one test the setting of a fresh process: stereo,
// not yet settled. Every pipeline with audio settles it, so a test that
// depends on it starts here.
func resetDeviceChannels(t *testing.T) {
	t.Helper()
	reset := func() {
		audioDevice.mu.Lock()
		audioDevice.channels, audioDevice.fixed = 0, false
		audioDevice.mu.Unlock()
	}
	reset()
	t.Cleanup(reset)
}

// stereoOnlyDevice is a device that refuses more than two channels, as a
// stereo-only output could, and records what it was asked for.
func stereoOnlyDevice(t *testing.T) (asked *[]int, refused error) {
	t.Helper()
	refused = errors.New("device refuses more than 2 channels")
	asked = new([]int)
	withAudioOutput(t, func(channels int) (*audio.Output, error) {
		*asked = append(*asked, channels)
		if channels > 2 {
			return nil, refused
		}
		return audio.NewDetachedOutput(channels), nil
	})
	return asked, refused
}

// TestDeviceChannelsDefaultsToStereo pins the owner's channel policy on #707:
// the device is stereo whatever the stream, until the app asks otherwise.
func TestDeviceChannelsDefaultsToStereo(t *testing.T) {
	resetDeviceChannels(t)
	if got := deviceChannels(); got != 2 {
		t.Errorf("deviceChannels() = %d, want 2", got)
	}
}

// TestASurroundStreamFirstLeavesTheDeviceStereo is #707: a 5.1 stream built
// first used to open the device at 6 channels, so on a device that refuses 6
// it failed, and since the device opens once per process, so did every later
// player, stereo ones included. Now both build, on a stereo device, and the 5.1
// stream's decoder is asked to downmix.
func TestASurroundStreamFirstLeavesTheDeviceStereo(t *testing.T) {
	resetDeviceChannels(t)
	asked, _ := stereoOnlyDevice(t)

	surround := newAudioSource(audio.DeviceSampleRate, 6)
	p1, err := buildWithin(t, func() (*Pipeline, error) { return newPipelineForSource(surround) })
	if err != nil {
		t.Fatalf("6-channel pipeline: %v", err)
	}
	defer p1.Close()
	if got := p1.audioOutput.Channels(); got != 2 {
		t.Errorf("6-channel stream: device has %d channels, want 2", got)
	}
	if surround.askedChannels != 2 {
		t.Errorf("6-channel stream: decoder asked for %d channels, want a downmix to 2", surround.askedChannels)
	}

	plain := newAudioSource(44100, 2)
	p2, err := buildWithin(t, func() (*Pipeline, error) { return newPipelineForSource(plain) })
	if err != nil {
		t.Fatalf("2-channel pipeline after a 6-channel one: %v", err)
	}
	defer p2.Close()

	if want := []int{2, 2}; len(*asked) != 2 || (*asked)[0] != 2 || (*asked)[1] != 2 {
		t.Errorf("device asked for %v, want %v", *asked, want)
	}
}

// TestSurroundIsOptIn: an app that sets the device's channels before the
// first pipeline gets them, and a stereo stream is upmixed to them.
func TestSurroundIsOptIn(t *testing.T) {
	resetDeviceChannels(t)
	var asked []int
	withAudioOutput(t, func(channels int) (*audio.Output, error) {
		asked = append(asked, channels)
		return audio.NewDetachedOutput(channels), nil
	})
	if err := SetDeviceChannels(6); err != nil {
		t.Fatalf("SetDeviceChannels(6) before any pipeline: %v", err)
	}

	plain := newAudioSource(audio.DeviceSampleRate, 2)
	p, err := buildWithin(t, func() (*Pipeline, error) { return newPipelineForSource(plain) })
	if err != nil {
		t.Fatalf("pipeline on a 6-channel device: %v", err)
	}
	defer p.Close()
	if len(asked) != 1 || asked[0] != 6 {
		t.Errorf("device asked for %v, want [6]", asked)
	}
	if plain.askedChannels != 6 {
		t.Errorf("stereo stream's decoder asked for %d channels, want 6", plain.askedChannels)
	}

	// The device is settled now: the same count is accepted, another refused.
	if err := SetDeviceChannels(6); err != nil {
		t.Errorf("SetDeviceChannels(6) again after the device opened at 6: %v", err)
	}
	if err := SetDeviceChannels(2); err == nil {
		t.Error("SetDeviceChannels(2) after the device opened at 6 returned nil; the change would be silently ignored")
	}
	if got := deviceChannels(); got != 6 {
		t.Errorf("deviceChannels() after a refused change = %d, want 6", got)
	}
}

// TestSetDeviceChannelsAfterTheDeviceOpenedIsAnError: setting surround after a
// stereo device opened is refused, not ignored.
func TestSetDeviceChannelsAfterTheDeviceOpenedIsAnError(t *testing.T) {
	resetDeviceChannels(t)
	withAudioOutput(t, detachedDevice)
	p, err := buildWithin(t, func() (*Pipeline, error) { return newPipelineForSource(newAudioSource(44100, 2)) })
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	if err := SetDeviceChannels(6); err == nil {
		t.Error("SetDeviceChannels(6) after a stereo device opened returned nil")
	}
	if err := SetDeviceChannels(2); err != nil {
		t.Errorf("SetDeviceChannels(2), the count already in use: %v", err)
	}
}

// TestSetDeviceChannelsBeforeAnyAudioIsFree: a pipeline without audio never
// opens the device, so it does not settle the count.
func TestSetDeviceChannelsBeforeAnyAudioIsFree(t *testing.T) {
	resetDeviceChannels(t)
	p := newTestPipeline(t, newShapedSource(16, 16)) // video only
	defer p.Close()
	if err := SetDeviceChannels(6); err != nil {
		t.Errorf("SetDeviceChannels(6) after a video-only pipeline: %v", err)
	}
}

func TestSetDeviceChannelsRefusesNonPositive(t *testing.T) {
	resetDeviceChannels(t)
	for _, n := range []int{0, -1} {
		if err := SetDeviceChannels(n); err == nil {
			t.Errorf("SetDeviceChannels(%d) returned nil", n)
		}
	}
	if got := deviceChannels(); got != 2 {
		t.Errorf("deviceChannels() after refused settings = %d, want 2", got)
	}
}

// TestOptedInSurroundOnARefusingDeviceFails pins the cost of the opt-in that
// SetAudioDeviceChannels documents: a device that refuses the count fails the
// pipeline, as any device that will not open does (#690).
func TestOptedInSurroundOnARefusingDeviceFails(t *testing.T) {
	resetDeviceChannels(t)
	_, refused := stereoOnlyDevice(t)
	if err := SetDeviceChannels(6); err != nil {
		t.Fatal(err)
	}
	_, err := buildWithin(t, func() (*Pipeline, error) { return newPipelineForSource(newAudioSource(44100, 2)) })
	if !errors.Is(err, refused) {
		t.Errorf("pipeline error = %v, want it to wrap %v", err, refused)
	}
}
