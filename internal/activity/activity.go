// Package activity records when the user last gave the app input, for
// [bellui.LastUserActivity] (#1108). The drivers call Stamp as they receive
// a touch, a click, a scroll or a key, before any widget sees it, so a scroll
// inside a list counts though the list reports nothing. It imports nothing
// from the module, so every driver can use it.
package activity

import (
	"sync/atomic"
	"time"
)

// base is read once, so Last can give back times that carry a monotonic
// reading: an idle timer comparing it with time.Now is not fooled by the wall
// clock being set.
var base = time.Now()

// last is the time of the last input as an offset from base, plus one so that
// zero means none yet. A stamp is one atomic store and allocates nothing, so
// the moves of a drag can each make one.
var last atomic.Int64

// Stamp records that the user gave input now.
func Stamp() { last.Store(int64(time.Since(base)) + 1) }

// Last returns the time of the last Stamp, or the zero time if there was none.
func Last() time.Time {
	d := last.Load()
	if d == 0 {
		return time.Time{}
	}
	return base.Add(time.Duration(d - 1))
}

// Reset forgets every stamp. Tests call it.
func Reset() { last.Store(0) }
