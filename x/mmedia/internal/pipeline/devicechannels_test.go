package pipeline

import "testing"

// TestDeviceChannels pins the owner's choice on #506: the device is stereo,
// except for surround played without WithDownmix, which keeps its layout.
func TestDeviceChannels(t *testing.T) {
	for _, c := range []struct {
		stream  int
		downmix bool
		want    int
	}{
		{1, false, 2}, // mono no longer opens a mono device
		{1, true, 2},
		{2, false, 2},
		{6, false, 6}, // 5.1 passes through, as WithDownmix documents
		{6, true, 2},
		{8, false, 8},
		{0, false, 2}, // a source that reports no layout
	} {
		if got := deviceChannels(c.stream, c.downmix); got != c.want {
			t.Errorf("deviceChannels(%d, downmix=%v) = %d, want %d", c.stream, c.downmix, got, c.want)
		}
	}
}
