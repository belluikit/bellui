package driver

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/core"
)

// plainObject is an object with no opacity of its own. The stack never calls
// its methods.
type plainObject struct{ core.CanvasObject }

// disabledObject is a disabled widget, as far as the stack can tell.
type disabledObject struct{ core.CanvasObject }

func (disabledObject) Disabled() bool { return true }
func (disabledObject) Enable()        {}
func (disabledObject) Disable()       {}

type recordedOpacity struct{ v float32 }

func (r *recordedOpacity) SetGroupOpacity(v float32) { r.v = v }
func (r *recordedOpacity) ClearGroupOpacity()        { r.v = 1 }

type fixedOpacity struct {
	core.CanvasObject
	v float32
}

func (f fixedOpacity) GroupOpacity() float32 { return f.v }

func TestOpacityStack(t *testing.T) {
	p := &recordedOpacity{v: 1}
	var s OpacityStack
	plain := plainObject{}

	s.Push(p, fixedOpacity{v: 0.5})
	assert.Equal(t, float32(0.5), p.v)
	s.Push(p, plain)
	assert.Equal(t, float32(0.5), p.v, "a plain object keeps its parent's")
	s.Push(p, fixedOpacity{v: 0.5})
	assert.Equal(t, float32(0.25), p.v, "nested groups multiply")
	s.Push(p, fixedOpacity{v: 7})
	assert.Equal(t, float32(0.25), p.v, "an opacity over 1 is 1")
	s.Pop(p)
	s.Pop(p)
	assert.Equal(t, float32(0.5), p.v, "a pop restores the parent's, not full opacity")

	s.EnterBoundary(p)
	assert.Equal(t, float32(1), p.v, "a boundary's cached content starts opaque")
	s.ExitBoundary(p)
	assert.Equal(t, float32(0.5), p.v)

	s.Push(p, fixedOpacity{v: -1})
	assert.True(t, s.Transparent(), "an opacity under 0 is 0")
	s.Pop(p)
	s.Pop(p)
	s.Pop(p)
	assert.Equal(t, float32(1), p.v)
	s.Pop(p) // an extra pop is harmless
	assert.Equal(t, float32(1), s.Current())
}

// ownLookObject is a disabled widget that draws its own disabled look.
type ownLookObject struct{ disabledObject }

func (ownLookObject) DrawsDisabledLook() bool { return true }

// notOwnLookObject implements DisabledLook but reports no look of its own.
type notOwnLookObject struct{ disabledObject }

func (notOwnLookObject) DrawsDisabledLook() bool { return false }

// enabledObject is an enabled widget.
type enabledObject struct{ disabledObject }

func (enabledObject) Disabled() bool { return false }

// Every walk dims a disabled widget, unless it draws its own disabled look
// (#1223). The desktop used to dim every disabled widget, so a Button, which
// draws MD3's disabled colours itself, was dimmed twice: its label at
// 0.38 x 0.38 of the text colour, measured on the GL driver.
func TestOpacityStack_DimsDisabledWithoutALookOfItsOwn(t *testing.T) {
	p := &recordedOpacity{v: 1}
	var s OpacityStack

	s.Push(p, enabledObject{})
	assert.Equal(t, float32(1), p.v, "an enabled widget")
	s.Pop(p)

	s.Push(p, ownLookObject{})
	assert.Equal(t, float32(1), p.v, "a widget with its own disabled look is not dimmed again")
	s.Pop(p)

	s.Push(p, notOwnLookObject{})
	assert.InDelta(t, 0.38, p.v, 1e-6, "DrawsDisabledLook false is dimmed")
	s.Pop(p)

	s.Push(p, disabledObject{})
	assert.InDelta(t, 0.38, p.v, 1e-6, "a disabled widget with no look of its own")
	s.Push(p, disabledObject{})
	assert.InDelta(t, 0.38*0.38, p.v, 1e-6, "a disabled widget inside another")
	s.Push(p, ownLookObject{})
	assert.InDelta(t, 0.38*0.38, p.v, 1e-6, "an ancestor's dim still covers a widget with its own look")
}

// countingOpacity counts the painter calls the stack makes.
type countingOpacity struct{ calls int }

func (c *countingOpacity) SetGroupOpacity(float32) { c.calls++ }
func (c *countingOpacity) ClearGroupOpacity()      { c.calls++ }

// The painter hears only of changes: a tree without opacity costs no calls.
func TestOpacityStack_OnlyChangesReachThePainter(t *testing.T) {
	p := &countingOpacity{}
	var s OpacityStack
	for range 3 {
		s.Push(p, plainObject{})
	}
	for range 3 {
		s.Pop(p)
	}
	assert.Zero(t, p.calls)

	s.Push(p, fixedOpacity{v: 0.5})
	s.Push(p, plainObject{})
	s.Pop(p)
	s.Pop(p)
	assert.Equal(t, 2, p.calls, "into the group and out of it")
}
