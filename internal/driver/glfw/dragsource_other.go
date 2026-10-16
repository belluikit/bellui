//go:build (!darwin || ios || wasm || js) && !windows && !linux && !freebsd && !openbsd && !netbsd

package glfw

import (
	"image"

	"github.com/belluikit/bellui/driver/desktop"
)

// startNativeDrag has no backend on this platform (macOS is in
// dragsource_darwin.go, Windows in dragsource_windows.go, Linux and the BSDs in
// dragsource_xdg.go/dragsource_x11.go — see docs/design/515-drag-out.md,
// pre-rebrand tracker #515 Part B), so a drag-out request is declined and the caller keeps its
// in-app behavior. This covers ios/wasm/js and any remaining desktop OS without
// a hand-rolled backend.
func startNativeDrag(desktop.DragData, image.Image) bool {
	return false
}
