//go:build !nopartialrepaint && (partialrepaint || (!android && !ios && !js && !wasm && !mobile))

package build

// partialRepaintDefault is whether a build repaints only the dirty region
// unless BELLUI_PARTIAL_REPAINT=0 says otherwise: on desktop, and wherever
// the partialrepaint tag asks for it (#496).
const partialRepaintDefault = true
