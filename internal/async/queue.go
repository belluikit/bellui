package async

import (
	"sync/atomic"

	"github.com/belluikit/bellui/core"
)

// CanvasObjectQueue implements lock-free FIFO freelist based queue.
//
// Reference: https://dl.acm.org/citation.cfm?doid=248052.248106
type CanvasObjectQueue struct {
	head atomic.Pointer[itemCanvasObject]
	tail atomic.Pointer[itemCanvasObject]
	len  atomic.Uint64
}

// NewCanvasObjectQueue returns a queue for caching values.
func NewCanvasObjectQueue() *CanvasObjectQueue {
	head := &itemCanvasObject{}
	queue := &CanvasObjectQueue{}
	queue.head.Store(head)
	queue.tail.Store(head)
	return queue
}

type itemCanvasObject struct {
	next atomic.Pointer[itemCanvasObject]
	v    core.CanvasObject
}

var itemCanvasObjectPool = Pool[*itemCanvasObject]{
	New: func() *itemCanvasObject { return &itemCanvasObject{} },
}

// In puts the given value at the tail of the queue.
func (q *CanvasObjectQueue) In(v core.CanvasObject) {
	i := itemCanvasObjectPool.Get()
	i.next.Store(nil)
	i.v = v

	var last, lastnext *itemCanvasObject
	for {
		last = q.tail.Load()
		lastnext = last.next.Load()
		if q.tail.Load() == last {
			if lastnext == nil {
				if last.next.CompareAndSwap(lastnext, i) {
					q.tail.CompareAndSwap(last, i)
					q.len.Add(1)
					return
				}
			} else {
				q.tail.CompareAndSwap(last, lastnext)
			}
		}
	}
}

// Out removes and returns the value at the head of the queue.
// It returns nil if the queue is empty. It may be called from one goroutine at
// a time: In from any number.
//
// The item it takes the value from becomes the queue's head, and it lets go of
// the value: a head that kept it held the last object refreshed, so the last
// screen an idle app showed, until the next refresh (#1075).
func (q *CanvasObjectQueue) Out() core.CanvasObject {
	var first, last, firstnext *itemCanvasObject
	for {
		first = q.head.Load()
		last = q.tail.Load()
		firstnext = first.next.Load()
		if first == q.head.Load() {
			if first == last {
				if firstnext == nil {
					return nil
				}

				q.tail.CompareAndSwap(last, firstnext)
			} else {
				v := firstnext.v
				if q.head.CompareAndSwap(first, firstnext) {
					firstnext.v = nil // only Out reads it, and In wrote it before linking
					q.len.Add(^uint64(0))
					itemCanvasObjectPool.Put(first)
					return v
				}
			}
		}
	}
}

// Len returns the length of the queue.
func (q *CanvasObjectQueue) Len() uint64 {
	return q.len.Load()
}
