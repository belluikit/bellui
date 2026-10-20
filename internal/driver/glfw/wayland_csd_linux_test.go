//go:build ((x11 && wayland) || (!x11 && !wayland)) && linux && !wasm

package glfw

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// forcePlatform follows the display the environment names (#534). The case
// with both displays set asks the compositor, so it is not covered here.
func TestForcePlatform(t *testing.T) {
	for name, tc := range map[string]struct {
		belluiPlatform, waylandDisplay, display string
		want                                    string
	}{
		"X display only picks X11":        {display: ":1", want: platformX11},
		"Wayland display only is auto":    {waylandDisplay: "wayland-0", want: platformAuto},
		"no display is auto":              {want: platformAuto},
		"BELLUI_PLATFORM=wayland wins":    {belluiPlatform: "wayland", display: ":1", want: platformWayland},
		"BELLUI_PLATFORM=x11 wins":        {belluiPlatform: "x11", waylandDisplay: "wayland-0", want: platformX11},
		"unknown BELLUI_PLATFORM ignored": {belluiPlatform: "mir", display: ":1", want: platformX11},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("BELLUI_PLATFORM", tc.belluiPlatform)
			t.Setenv("WAYLAND_DISPLAY", tc.waylandDisplay)
			t.Setenv("DISPLAY", tc.display)
			assert.Equal(t, tc.want, forcePlatform())
		})
	}
}
