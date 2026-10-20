package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestSetCanvasForObjectMoved pins #448: an object registered to one canvas
// and walked on another, with the first still open, moves to the second. It
// used to keep the first, so refreshes went to the window it had left and
// every walk set it up again.
func TestSetCanvasForObjectMoved(t *testing.T) {
	a, b := &dummyCanvas{}, &dummyCanvas{}
	obj := &dummyWidget{}
	called := 0
	setup := func() { called++ }

	regA := RegisterCanvasForObject(obj, a, setup)
	require.Equal(t, a, GetCanvasForObject(obj))

	regB := RegisterCanvasForObject(obj, b, setup)
	assert.Equal(t, b, GetCanvasForObject(obj), "maps to the canvas it moved to")
	assert.Equal(t, 2, called, "set up once for the move")
	assert.True(t, regB.Current(b))
	assert.False(t, regA.Current(a), "the replaced registration is not current: a walk holding it registers again")

	RegisterCanvasForObject(obj, b, setup)
	RegisterCanvasForObject(obj, b, setup)
	assert.Equal(t, 2, called, "no setup again once registered here")

	RegisterCanvasForObject(obj, a, setup)
	assert.Equal(t, a, GetCanvasForObject(obj), "moving back moves it back")
	assert.Equal(t, 3, called)
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

// TestCanvasRegistrationCurrent pins the rule the min-size walk relies on to
// skip looking an object up (#487): a registration is current only while the
// map still holds that entry for that canvas, so once Clean deletes an expired
// entry the walk registers the object again and GetCanvasForObject finds it.
func TestCanvasRegistrationCurrent(t *testing.T) {
	testClearAll()
	t.Cleanup(testClearAll)
	tm := &timeMock{}
	tm.setTime(10, 0)

	c := &dummyCanvas{}
	obj := &dummyWidget{}
	reg := RegisterCanvasForObject(obj, c, nil)
	assert.True(t, reg.Current(c))
	assert.False(t, reg.Current(&dummyCanvas{}), "current for another canvas")
	assert.False(t, CanvasRegistration{}.Current(c), "a zero registration is current")

	// Past its expiry, and after Clean has deleted it, it is not current.
	tm.setTime(11, 1)
	assert.False(t, reg.Current(c), "an expired registration is current")
	Clean(true)
	assert.Nil(t, GetCanvasForObject(obj), "Clean kept the expired entry")

	// Registering again restores the mapping and a current registration.
	reg = RegisterCanvasForObject(obj, c, nil)
	assert.True(t, reg.Current(c))
	assert.Equal(t, c, GetCanvasForObject(obj))
}
