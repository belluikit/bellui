package uidir

import (
	"runtime"
	"sync"
	"testing"
	"weak"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/thread"
)

func TestCurrent_Empty(t *testing.T) {
	d, ok := Current()
	assert.False(t, ok)
	assert.Equal(t, bellui.DirectionAuto, d)
}

func TestPushPop(t *testing.T) {
	Push(bellui.DirectionRTL)
	d, ok := Current()
	assert.True(t, ok)
	assert.Equal(t, bellui.DirectionRTL, d)

	Pop(bellui.DirectionRTL)
	_, ok = Current()
	assert.False(t, ok)
}

func TestPushPop_Nested(t *testing.T) {
	Push(bellui.DirectionRTL)
	defer Pop(bellui.DirectionRTL)

	Push(bellui.DirectionLTR)
	d, ok := Current()
	assert.True(t, ok)
	assert.Equal(t, bellui.DirectionLTR, d, "innermost scope wins")
	Pop(bellui.DirectionLTR)

	d, ok = Current()
	assert.True(t, ok)
	assert.Equal(t, bellui.DirectionRTL, d, "outer scope restored")
}

func TestPushPop_AutoIsNoop(t *testing.T) {
	Push(bellui.DirectionRTL)
	defer Pop(bellui.DirectionRTL)

	Push(bellui.DirectionAuto) // must not shadow the enclosing RTL scope
	d, ok := Current()
	assert.True(t, ok)
	assert.Equal(t, bellui.DirectionRTL, d)
	Pop(bellui.DirectionAuto)

	d, ok = Current()
	assert.True(t, ok)
	assert.Equal(t, bellui.DirectionRTL, d)
}

func TestMarkRestore_DiscardsLeakedEntries(t *testing.T) {
	mark := Mark()
	assert.Equal(t, 0, mark)

	// Simulate a walk that pushed per-node scopes but panicked before the
	// matching Pops ran, leaving entries on the stack.
	Push(bellui.DirectionRTL)
	Push(bellui.DirectionLTR)
	d, ok := Current()
	assert.True(t, ok)
	assert.Equal(t, bellui.DirectionLTR, d)

	Restore(mark)
	_, ok = Current()
	assert.False(t, ok, "Restore truncates the stack back to the mark")
}

func TestMarkRestore_NestedKeepsOuter(t *testing.T) {
	Push(bellui.DirectionRTL)
	defer Pop(bellui.DirectionRTL)

	mark := Mark()
	assert.Equal(t, 1, mark)

	Push(bellui.DirectionLTR) // leaked inner scope
	Restore(mark)

	d, ok := Current()
	assert.True(t, ok)
	assert.Equal(t, bellui.DirectionRTL, d, "outer scope below the mark survives")
}

func TestRestore_BalancedIsNoop(t *testing.T) {
	mark := Mark()
	Push(bellui.DirectionRTL)
	Pop(bellui.DirectionRTL) // already balanced

	Restore(mark) // must not panic or corrupt state
	_, ok := Current()
	assert.False(t, ok)
}

func TestPerGoroutineIsolation(t *testing.T) {
	Push(bellui.DirectionRTL)
	defer Pop(bellui.DirectionRTL)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, ok := Current()
		assert.False(t, ok, "another goroutine sees no override")
	}()
	wg.Wait()
}

type overlayObj struct {
	core.CanvasObject
	name string
}

// An overlay that never leaves its stack through Remove (its window closed
// with the popup open) is not kept alive by its inherited direction, and its
// entry goes once it is collected (#749).
func TestSetInherited_HoldsTheOverlayWeakly(t *testing.T) {
	ov := &overlayObj{name: "popup"}
	SetInherited(ov, bellui.DirectionRTL)
	d, ok := Inherited(ov)
	require.True(t, ok)
	require.Equal(t, bellui.DirectionRTL, d)
	require.Contains(t, Scopes(), core.CanvasObject(ov))
	wp := weak.Make(ov)
	ov = nil

	for range 3 {
		runtime.GC()
	}
	assert.Nil(t, wp.Value(), "the entry did not keep the overlay alive")
	Scopes() // prunes
	assert.Nil(t, inheritedList.Load(), "the collected overlay's entry is gone")
}

// PopOwned pops what its owner pushed, even when the owner's direction
// changed in between, and nothing for an owner that pushed nothing (#749).
func TestPopOwned_PopsWhatTheOwnerPushed(t *testing.T) {
	ov, other := &overlayObj{name: "a"}, &overlayObj{name: "b"}
	Push(bellui.DirectionLTR)
	defer Pop(bellui.DirectionLTR)

	PushOwned(bellui.DirectionRTL, ov, true)
	PopOwned(other, false) // pushed nothing
	d, _ := Current()
	assert.Equal(t, bellui.DirectionRTL, d, "another object's pop leaves ov's entry")
	PopOwned(ov, false)
	d, _ = Current()
	assert.Equal(t, bellui.DirectionLTR, d, "ov's pop removes its own entry only")
	assert.Zero(t, overlayPushes.Load())
}

// On the main goroutine a push and its pop allocate nothing once the stack
// exists: a Refresh outside a walk does one of each per widget (#772).
func TestPushPop_MainGoroutineDoesNotAllocate(t *testing.T) {
	thread.SetMainGoroutine()
	Push(bellui.DirectionRTL)
	Pop(bellui.DirectionRTL)

	allocs := testing.AllocsPerRun(100, func() {
		Push(bellui.DirectionRTL)
		Pop(bellui.DirectionRTL)
	})
	assert.Zero(t, allocs)
}

// Another goroutine's stack is dropped once it empties, so a goroutine that
// pushed and ended leaves nothing in goroutineStacks.
func TestPushPop_OtherGoroutineLeavesNoStack(t *testing.T) {
	var id uint64
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		id = thread.GoroutineID()
		Push(bellui.DirectionRTL)
		PushOwned(bellui.DirectionLTR, nil, false)
		PopTop()
		Pop(bellui.DirectionRTL)
	}()
	wg.Wait()
	_, ok := goroutineStacks.Load(id)
	assert.False(t, ok)
}
