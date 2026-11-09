package embedded_test

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/driver/embedded"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/activity"
	intNoos "github.com/belluikit/bellui/internal/driver/embedded"
)

// Every embedded event stamps the time of the last user activity, a finger
// moving on its own included: that is how a list is scrolled (#1108).
func TestNoOSDriver_InputStampsUserActivity(t *testing.T) {
	t.Cleanup(activity.Reset)
	for name, ev := range map[string]embedded.Event{
		"touch down": &embedded.TouchDownEvent{Position: geo.NewPos(10, 10), ID: 1},
		"touch move": &embedded.TouchMoveEvent{Position: geo.NewPos(10, 40), ID: 1},
		"key":        &embedded.KeyEvent{Name: event.KeyA},
		"key up":     &embedded.KeyEvent{Name: event.KeyA, Direction: embedded.KeyReleased},
		"character":  &embedded.CharacterEvent{Rune: 'a'},
	} {
		t.Run(name, func(t *testing.T) {
			queue := make(chan embedded.Event, 1)
			size := func() geo.Size { return geo.NewSquareSize(100) }
			d := intNoos.NewNoOSDriver(func(image.Image) {}, func(f func()) { f() }, queue, size)
			_ = d.CreateWindow("Test")
			activity.Reset()

			queue <- ev
			close(queue)
			d.Run()

			assert.False(t, activity.Last().IsZero(), "%s counts as user activity", name)
		})
	}
}
