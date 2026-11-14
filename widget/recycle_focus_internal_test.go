package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/internal/apphook"
)

// A widget can be disabled before any app exists, when there is no canvas to
// hold a focus: unfocusInside must then do nothing, rather than ask
// bellui.CurrentApp, which logs an error and returns nil (#1221).
func TestUnfocusInside_BeforeApp(t *testing.T) {
	if apphook.Started() {
		prev := bellui.CurrentApp()
		t.Cleanup(func() { bellui.SetCurrentApp(prev) })
	}
	bellui.SetCurrentApp(nil)
	assert.NotPanics(t, func() { unfocusInside(canvas.NewRectangle(nil)) })
}
