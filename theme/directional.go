package theme

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/svg"
	"github.com/belluikit/bellui/lang"
)

// directionalResource wraps a directional icon — a back/forward arrow, a
// submenu chevron, a reply/forward glyph — so that its artwork is mirrored
// left-to-right whenever the active user-interface direction is right-to-left.
// Non-directional icons (search, settings, a clock) are never wrapped and so
// keep their orientation under every locale.
//
// Which icons are directional follows Material Symbols: an icon is wrapped
// when the Android drawable of the same Material symbol is marked
// android:autoMirrored="true" (#779). That takes in undo and redo, send, the
// list and file icons, login and logout, help, copy and the volume icons,
// and leaves out the media controls. TestDirectionalResource_Set pins the
// whole classification, so a new icon is classified on purpose.
//
// The direction is resolved lazily inside [directionalResource.Name] and
// [directionalResource.Content] (via [lang.IsRTLDirection]) rather than captured
// once, so a runtime direction change (see
// [github.com/belluikit/bellui.SetLayoutDirection]) is honoured on the next
// render: under RTL the wrapper reports a distinct Name — giving the mirrored
// artwork its own entry in the SVG raster cache — and returns flipped Content,
// while the Phase 3 direction-change repaint re-reads both.
type directionalResource struct {
	source bellui.Resource
}

// newDirectionalResource marks src as a directional icon that mirrors under a
// right-to-left layout.
func newDirectionalResource(src bellui.Resource) *directionalResource {
	return &directionalResource{source: src}
}

// Name returns the source name, prefixed with "rtl_" under a right-to-left
// layout so the mirrored variant caches separately from the upright one.
func (res *directionalResource) Name() string {
	if lang.IsRTLDirection() {
		return "rtl_" + res.source.Name()
	}
	return res.source.Name()
}

// Content returns the source artwork, mirrored horizontally under a
// right-to-left layout and returned unchanged otherwise.
func (res *directionalResource) Content() []byte {
	if lang.IsRTLDirection() {
		return svg.FlipHorizontal(res.source.Content())
	}
	return res.source.Content()
}
