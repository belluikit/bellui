//go:build !linux || android || !cgo

package webview

import (
	"testing"

	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
	"github.com/stretchr/testify/require"
)

// The render tree must contain the NativeViewHost ITSELF, never an adapter
// wrapping it. The host's per-frame sync finds itself in the tree by identity
// (Driver.CanvasForObject, driver.GeometryForObject), so a wrapper standing
// in for it is never matched: the platform view then never attaches, on every
// hole-punch platform, with nothing logged — the regression #82's interactive
// pass caught after #105 introduced the portable Host.
func TestHostRendererShowsTheNativeViewHostItself(t *testing.T) {
	test.NewTempApp(t)

	host, _ := NewHost("")
	objects := test.TempWidgetRenderer(t, host).Objects()
	require.Len(t, objects, 1)
	_, ok := objects[0].(*widget.NativeViewHost)
	require.True(t, ok,
		"the tree object is %T, want *widget.NativeViewHost by identity — "+
			"a wrapper breaks the host's self-referential tree walks", objects[0])
}
