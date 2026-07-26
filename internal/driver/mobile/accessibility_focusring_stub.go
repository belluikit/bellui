//go:build !accessibility

package mobile

import "github.com/belluikit/bellui/geo"

// a11yFocusRing is a no-op without the accessibility bridge: there is no screen
// reader focus to track, so the in-scene focus ring (#759) is never drawn.
func (w *window) a11yFocusRing() (geo.Rectangle, bool) {
	return geo.Rectangle{}, false
}
