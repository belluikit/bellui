package common

import (
	"math"
	"sync/atomic"
	"time"

	"github.com/belluikit/bellui/event"
)

// DragCoast carries a released touch drag on with decaying deltas
// (DragDecay, about 60 steps a second) and then ends it, for the drivers that
// give touch drags momentum (mobile, embedded) — only for Draggables that ask
// for it (event.MomentumDraggable, #751). A driver keeps one and calls
// Stop before it delivers any new touch, so a touch during a coast stops it
// dead (#751).
//
// Release and Stop run on the main goroutine. The coast's own goroutine only
// sleeps and queues steps: each step, and the final DragEnd, runs on the main
// goroutine through the driver's queue and checks there that its coast is
// still the current one. Since Stop runs on the main goroutine too, a step
// queued before a Stop but run after it finds its coast replaced and does
// nothing, so no step and no DragEnd from a stopped coast reaches the
// Draggable after the new touch went down.
type DragCoast struct {
	// gen numbers coasts. It is written on the main goroutine only; the
	// coast goroutine reads it to stop queueing steps nobody will run.
	gen atomic.Uint64
	// wid is the Draggable coasting now, nil when none is. Main goroutine only.
	wid event.Draggable
}

// Release ends a touch drag released with ev on wid, whose Dragged is the
// momentum seed (TouchHandler.TapUp sets it from the finger's speed at
// release). Only a Draggable that opts in through event.MomentumDraggable
// coasts (#751); any other, and a slow release, ends the drag at once. A
// coasting one queues each step and the final DragEnd with do, which must run
// its function on the main goroutine.
func (c *DragCoast) Release(wid event.Draggable, ev *event.DragEvent, do func(func())) {
	c.Stop()
	if m, ok := wid.(event.MomentumDraggable); !ok || !m.DragMomentum() {
		wid.DragEnd() // no momentum is the default: stop where the finger lifted
		return
	}
	if math.Abs(float64(ev.Dragged.DX)) <= TapMoveEndThreshold && math.Abs(float64(ev.Dragged.DY)) <= TapMoveEndThreshold {
		wid.DragEnd()
		return
	}

	gen := c.gen.Add(1)
	c.wid = wid
	start := *ev // each step gets its own event: the Draggable may keep it
	go func() {
		delta := start.Dragged
		for math.Abs(float64(delta.DX)) > TapMoveEndThreshold || math.Abs(float64(delta.DY)) > TapMoveEndThreshold {
			if c.gen.Load() != gen {
				return
			}
			delta = DragDecay(delta)
			step := delta
			do(func() {
				if c.gen.Load() != gen {
					return
				}
				ev := start
				ev.Dragged = step
				wid.Dragged(&ev)
			})
			time.Sleep(time.Millisecond * 16)
		}

		do(func() {
			if c.gen.Load() != gen {
				return
			}
			c.wid = nil
			c.gen.Add(1)
			wid.DragEnd()
		})
	}()
}

// Stop ends a running coast now: its Draggable gets DragEnd before Stop
// returns, so before whatever the caller dispatches next, and nothing the
// coast already queued reaches it afterwards. Without a coast it does
// nothing.
func (c *DragCoast) Stop() {
	wid := c.wid
	if wid == nil {
		return
	}
	c.wid = nil
	c.gen.Add(1)
	wid.DragEnd()
}
