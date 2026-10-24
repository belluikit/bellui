package mmedia

import "testing"

// TestSetAudioDeviceChannelsRefusesNonPositive: a count the device cannot be
// opened with is refused where it is set, not at the first player. The opt-in
// itself, and its refusal once the device is open, are tested in
// internal/pipeline, which can give each test a fresh process's setting.
func TestSetAudioDeviceChannelsRefusesNonPositive(t *testing.T) {
	if err := SetAudioDeviceChannels(0); err == nil {
		t.Error("SetAudioDeviceChannels(0) returned nil")
	}
}
