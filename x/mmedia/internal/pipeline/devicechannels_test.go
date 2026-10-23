package pipeline

import "testing"

// TestDeviceChannels pins the owner's choice on #506: the device is stereo,
// except for surround played without WithDownmix, which keeps its layout.
// Surround is any layout above stereo, as on main and as WithDownmix says
// ("the stream's native channel layout is passed through"), not only 5.1 and
// 7.1. Mono is played as stereo here and only here: NewOutput opens what it is
// asked for.
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
		{3, false, 3}, // 2.1
		{4, false, 4}, // quad
		{4, true, 2},
		{5, false, 5}, // 5.0
		{0, false, 2}, // a source that reports no layout
	} {
		if got := deviceChannels(c.stream, c.downmix); got != c.want {
			t.Errorf("deviceChannels(%d, downmix=%v) = %d, want %d", c.stream, c.downmix, got, c.want)
		}
	}
}
