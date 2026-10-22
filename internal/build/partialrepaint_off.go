//go:build nopartialrepaint || (!partialrepaint && (js || wasm || mobile || (gl && (android || ios))))

package build

// partialRepaintDefault is false where partial repaint is not yet validated,
// or where the nopartialrepaint tag turns it off: the drivers then clear and
// redraw the whole canvas every frame. Not yet validated: the web (#648), the
// GL painter on Android and iOS (-tags gl; the device checks in #593 ran on
// WebGPU), and the mobile driver built for a desktop with -tags mobile, which
// has no device surface to check.
const partialRepaintDefault = false
