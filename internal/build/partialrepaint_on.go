//go:build !nopartialrepaint && (partialrepaint || (!js && !wasm && !mobile && !(gl && (android || ios))))

package build

// partialRepaintDefault is whether a build repaints only the dirty region
// unless BELLUI_PARTIAL_REPAINT=0 says otherwise: on desktop (#496), on
// Android and iOS with the default WebGPU painter (#593), and wherever the
// partialrepaint tag asks for it.
const partialRepaintDefault = true
