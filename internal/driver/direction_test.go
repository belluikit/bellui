package driver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/uidir"
)

// A walk pops what an overlay pushed at its before-children callback, even
// when the overlay's inherited direction was dropped or set while the walk was
// inside it (a popup dismissed or re-shown from a layout): the stack stays
// balanced for the siblings that follow (#749).
func TestPushPopDirectionScope_InheritanceChangedInsideTheWalk(t *testing.T) {
	root := bellui.DirectionLTR
	uidir.Push(root)
	defer uidir.Pop(root)

	dropped := canvas.NewRectangle(nil)
	uidir.SetInherited(dropped, bellui.DirectionRTL)
	driver.PushDirectionScope(dropped)
	uidir.ForgetInherited(dropped)
	driver.PopDirectionScope(dropped)
	d, ok := uidir.Current()
	assert.True(t, ok, "the root push survives")
	assert.Equal(t, root, d, "the overlay's RTL does not outlive its subtree")

	set := canvas.NewRectangle(nil)
	driver.PushDirectionScope(set) // pushes nothing: no inherited direction yet
	uidir.SetInherited(set, bellui.DirectionRTL)
	driver.PopDirectionScope(set)
	uidir.ForgetInherited(set)
	d, ok = uidir.Current()
	assert.True(t, ok, "the pop of an overlay that pushed nothing leaves the root push")
	assert.Equal(t, root, d)
}

// An overlay that inherited a direction lays its content out in that
// direction when refreshed under another ambient one (#749).
func TestInDirectionOf_InheritedOverlayPushesItsOwn(t *testing.T) {
	popup := canvas.NewRectangle(nil)
	uidir.SetInherited(popup, bellui.DirectionRTL)
	defer uidir.ForgetInherited(popup)
	uidir.Push(bellui.DirectionLTR)
	defer uidir.Pop(bellui.DirectionLTR)

	var inside bellui.Direction
	driver.InDirectionOf(popup, func() { inside, _ = uidir.Current() })
	assert.Equal(t, bellui.DirectionRTL, inside)
}
