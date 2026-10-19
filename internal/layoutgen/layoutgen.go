// Package layoutgen counts changes to what layout depends on that refresh
// nothing: a min size set directly (SetMinSize) or a visibility change (Show,
// Hide) on primitives, containers and spacers. No canvas hears of them
// otherwise, so the next min-size pass of every canvas walks and asks every
// object again when the count moved (#464, #487). It imports nothing, so the
// root package can use it.
package layoutgen

import "sync/atomic"

var epoch atomic.Uint64

// Changed records such a change.
func Changed() { epoch.Add(1) }

// Epoch returns the count Changed raises.
func Epoch() uint64 { return epoch.Load() }
