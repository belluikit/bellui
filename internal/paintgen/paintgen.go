// Package paintgen counts changes to what a canvas paints that no canvas can
// place: an object shown (a hidden object has no node in any render tree, so
// no canvas knows where it would appear) or given a transform. The next
// partial-repaint region of every canvas then walks everything, as it did for
// every frame before moves were notified (#599). It imports nothing, so the
// root package can use it.
package paintgen

import "sync/atomic"

var epoch atomic.Uint64

// Changed records such a change.
func Changed() { epoch.Add(1) }

// Epoch returns the count Changed raises.
func Epoch() uint64 { return epoch.Load() }
