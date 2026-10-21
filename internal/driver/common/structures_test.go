package common

import (
	"runtime"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/async"
	"github.com/belluikit/bellui/layout"
	"github.com/stretchr/testify/assert"
)

func TestDeduplicatedObjectQueue(t *testing.T) {
	queue := deduplicatedObjectQueue{queue: async.NewCanvasObjectQueue()}
	assert.Zero(t, queue.Len())
	assert.Nil(t, queue.Out())

	obj1 := &bellui.Container{Layout: layout.NewCenterLayout()}
	queue.In(obj1)
	assert.Equal(t, uint64(1), queue.Len())
	queue.In(obj1)
	assert.Equal(t, uint64(1), queue.Len())

	obj2 := &bellui.Container{Layout: layout.NewFormLayout()}
	queue.In(obj2)
	assert.Equal(t, uint64(2), queue.Len())

	assert.Equal(t, obj1, queue.Out())
	assert.Equal(t, obj2, queue.Out())
	assert.Equal(t, nil, queue.Out())
	assert.Zero(t, queue.Len())

	queue.In(obj1)
	assert.Equal(t, uint64(1), queue.Len())
	queue.In(obj1)
	assert.Equal(t, uint64(1), queue.Len())
}

// TestDeduplicatedObjectQueueConcurrentInOut pins that a refresh queued from
// one goroutine while the main thread drains the queue is never lost (#445).
// In once queued the object before recording it, so an Out in between deleted
// a record not yet stored; the late store then marked the object queued for
// good and every later In for it returned early, so it never repainted.
func TestDeduplicatedObjectQueueConcurrentInOut(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skip("wasm runs one thread: the drain loop never yields to the producer, and In and Out cannot overlap")
	}
	queue := deduplicatedObjectQueue{queue: async.NewCanvasObjectQueue()}
	obj := &bellui.Container{}
	for round := 0; round < 5000; round++ {
		done := make(chan struct{})
		go func() {
			defer close(done)
			for i := 0; i < 500; i++ {
				queue.In(obj)
			}
		}()
	drain:
		for {
			select {
			case <-done:
				break drain
			default:
				queue.Out()
			}
		}
		for queue.Out() != nil {
		}

		queue.In(obj)
		if !assert.Same(t, obj, queue.Out(), "round %d: a refresh queued after the drain was dropped", round) {
			return
		}
	}
}

// TestDeduplicatedObjectQueueAllocations pins that a warmed-up queue refreshes
// without allocating: a labels frame queues thousands of refreshes, and the
// sync.Map this replaced allocated a node for each.
func TestDeduplicatedObjectQueueAllocations(t *testing.T) {
	if raceEnabled {
		t.Skip("the queue's items come from a sync.Pool, which -race empties at random")
	}
	queue := deduplicatedObjectQueue{queue: async.NewCanvasObjectQueue()}
	objs := make([]core.CanvasObject, 64)
	for i := range objs {
		objs[i] = &bellui.Container{}
	}
	cycle := func() {
		for _, o := range objs {
			queue.In(o)
			queue.In(o)
		}
		for queue.Out() != nil {
		}
	}
	cycle() // grow the map and fill the item pool
	assert.Zero(t, testing.AllocsPerRun(20, cycle))
}

func TestOverlayStack(t *testing.T) {
	stack := overlayStack{}

	obj1 := &bellui.Container{Layout: layout.NewCenterLayout()}
	stack.Add(obj1)
	assert.Equal(t, 1, len(stack.List()))
	assert.Equal(t, 1, len(stack.renderCaches))

	obj2 := &bellui.Container{Layout: layout.NewFormLayout()}
	stack.Add(obj2)
	assert.Equal(t, 2, len(stack.List()))
	assert.Equal(t, 2, len(stack.renderCaches))

	stack.Remove(obj1)
	assert.Zero(t, len(stack.List()))
	assert.Zero(t, len(stack.renderCaches))

	stack.Remove(obj2)
	assert.Zero(t, len(stack.List()))
	assert.Zero(t, len(stack.renderCaches))

	stack.Add(nil)
	assert.Zero(t, len(stack.List()))
	assert.Zero(t, len(stack.renderCaches))

	stack.Remove(nil)
	assert.Zero(t, len(stack.List()))
	assert.Zero(t, len(stack.renderCaches))
}
