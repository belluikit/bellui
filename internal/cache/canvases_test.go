package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetCanvasForObject(t *testing.T) {
	c := &dummyCanvas{}
	obj := &dummyWidget{}

	called := 0
	setup := func() {
		called++
	}

	SetCanvasForObject(obj, c, setup)
	assert.Equal(t, 1, called)
	SetCanvasForObject(obj, c, setup)
	assert.Equal(t, 1, called)

	// a different canvas (object moved window)
	c = &dummyCanvas{}
	SetCanvasForObject(obj, c, setup)
	assert.Equal(t, 2, called)
}

// TestSetCanvasForObjectRegisteredDoesNotAllocate: the paint walk calls this
// for every object every frame, so the already-registered case must not
// allocate (#435).
func TestSetCanvasForObjectRegisteredDoesNotAllocate(t *testing.T) {
	c := &dummyCanvas{}
	obj := &dummyWidget{}
	SetCanvasForObject(obj, c, nil)

	assert.Zero(t, testing.AllocsPerRun(100, func() { SetCanvasForObject(obj, c, nil) }))
}
