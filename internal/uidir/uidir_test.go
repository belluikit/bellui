package uidir

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
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
