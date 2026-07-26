package widget

import (
	"github.com/belluikit/bellui/core"
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
func mirrorIfRTL(objects []core.CanvasObject, width float32) {
	if lang.IsRTLDirection() {
		layout.MirrorChildren(objects, width)
	}
}
