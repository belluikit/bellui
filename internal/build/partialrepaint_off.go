//go:build nopartialrepaint || (!partialrepaint && (android || ios || js || wasm || mobile))

package build

// partialRepaintDefault is false where partial repaint is not yet validated
// (the mobile driver, the web), or where the nopartialrepaint tag turns it
// off: the drivers then clear and redraw the whole canvas every frame.
const partialRepaintDefault = false
