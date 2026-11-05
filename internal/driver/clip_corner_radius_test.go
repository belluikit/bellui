package driver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/driver"
)

// A container's ClipCornerRadius makes its ClipToBounds clip a rounded one
// (#991, player#268), and does nothing without ClipToBounds.
func TestContainerClipCornerRadius(t *testing.T) {
	for _, c := range []struct {
		name        string
		clip        bool
		radius      float32
		wantRounded bool
	}{
		{"square clip", true, 0, false},
		{"rounded clip", true, 24, true},
		{"radius without a clip", false, 24, false},
	} {
		cont := &bellui.Container{ClipToBounds: c.clip, ClipCornerRadius: c.radius}
		clips := driver.ClipsOf(cont)
		assert.Equal(t, c.clip, clips.Rect, "%s: Rect", c.name)
		assert.Equal(t, c.wantRounded, clips.Rounded, "%s: Rounded", c.name)
		rounded, radius := driver.IsRoundedClip(cont)
		assert.Equal(t, c.wantRounded, rounded, "%s: IsRoundedClip", c.name)
		if c.wantRounded {
			assert.Equal(t, c.radius, clips.Radius, "%s: Radius", c.name)
			assert.Equal(t, c.radius, radius, "%s: IsRoundedClip radius", c.name)
		}
	}
}
