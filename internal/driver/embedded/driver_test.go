package embedded_test

import (
	"image"
	"testing"
	"time"

	"github.com/belluikit/bellui/driver/embedded"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	intNoos "github.com/belluikit/bellui/internal/driver/embedded"
	"github.com/stretchr/testify/assert"
)

func TestNoOSDriver(t *testing.T) {
	count := 0
	render := func(img image.Image) {
		count++
	}
	queue := make(chan embedded.Event, 1)
	size := func() geo.Size {
		return geo.NewSquareSize(100)
	}
	d := intNoos.NewNoOSDriver(render, func(f func()) { f() }, queue, size)

	go func() {
		time.Sleep(100 * time.Millisecond)
		close(queue)
	}()
	d.Run()
	assert.Equal(t, 0, count)

	queue = make(chan embedded.Event, 1)
	d = intNoos.NewNoOSDriver(render, func(f func()) { f() }, queue, size)
	_ = d.CreateWindow("Test")
	go func() {
		time.Sleep(100 * time.Millisecond)
		close(queue)
	}()
	d.Run()
	assert.Equal(t, 1, count)

	count = 0
	queue = make(chan embedded.Event, 1)
	d = intNoos.NewNoOSDriver(render, func(f func()) { f() }, queue, size)
	w := d.CreateWindow("Test")
	keyed := make(chan bool)
	w.Canvas().SetOnTypedKey(func(k *event.KeyEvent) {
		keyed <- true
	})
	go func() {
		queue <- &embedded.CharacterEvent{
			Rune: 'a',
		}
		queue <- &embedded.KeyEvent{
			Name: event.KeyEscape,
		}
		<-keyed
		close(queue)
	}()
	d.Run()
	assert.Equal(t, 3, count)
}

func TestNoOSWindow_RequestFocus(t *testing.T) {
	count := 0
	render := func(image.Image) { count++ }
	queue := make(chan embedded.Event, 1)
	size := func() geo.Size { return geo.NewSquareSize(100) }
	d := intNoos.NewNoOSDriver(render, func(f func()) { f() }, queue, size)

	w := d.CreateWindow("Test")
	count = 0
	// RequestFocus must not panic and should render the focused window (#293).
	assert.NotPanics(t, w.RequestFocus)
	assert.Equal(t, 1, count, "RequestFocus renders the now-focused window")
}
