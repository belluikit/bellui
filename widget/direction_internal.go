package widget

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/uidir"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/layout"
)

// mirrorIfRTL reflects each child horizontally within width when the layout
// direction is right-to-left. It is the custom-renderer analogue of the standard
// layouts' MirrorChildren post-pass: a widget whose renderer positions a
// horizontal sequence of children from a left origin can call this at the end of
// its Layout to obtain the mirrored arrangement, leaving the left-to-right code
// path byte-for-byte unchanged.
//
// Full-width children (backgrounds, dividers, ripple overlays) reflect onto
// themselves, so a blanket pass over all of a renderer's objects is safe.
//
// It reads the ambient direction, which is right inside a layout walk. A
// renderer that lays itself out from its own Refresh runs outside one, so it
// wraps that Layout in [inDirectionOf].
func mirrorIfRTL(objects []core.CanvasObject, width float32) {
	if lang.IsRTLDirection() {
		layout.MirrorChildren(objects, width)
	}
}

// isRTLAt reports whether obj sits in a right-to-left layout (see
// driver.IsRTLAt). Event handlers use it instead of lang.IsRTLDirection, which
// sees only the global direction outside a layout or paint walk (#696). It
// looks obj up in its canvas' tree, so it is for once-per-event use, not per
// frame.
func isRTLAt(obj core.CanvasObject) bool { return driver.IsRTLAt(obj) }

// inDirectionOf runs f with obj's direction as the ambient one, so a layout f
// performs outside a layout walk — a renderer's Refresh re-laying itself out
// after a tap, a timer or a data binding — mirrors as the walk would (#696).
// Once a scope has been rendered it pushes the resolved direction even when
// that is the global one, so the lookups of children refreshed inside f return
// at once instead of walking. Before then nothing is pushed unless obj's
// canvas has a direction of its own: pushing would switch off uidir's
// no-scope fast path for every text measurement inside f.
func inDirectionOf(obj core.CanvasObject, f func()) {
	d, ok := driver.DirectionOf(obj)
	if !ok {
		if !uidir.ScopesInUse() {
			f() // the global direction applies, and lookups inside f are cheap
			return
		}
		d = driver.GlobalDirection()
	}
	uidir.Push(d)
	defer uidir.Pop(d)
	f()
}
