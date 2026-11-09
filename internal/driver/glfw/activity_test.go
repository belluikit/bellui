//go:build !no_glfw && !mobile

// No !ci, like tab_escape_test.go: the process methods need no GLFW window, so
// this runs in the browser leg of CI too, where the web driver calls them.

package glfw

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/activity"
	"github.com/belluikit/bellui/widget"
)

// Every kind of deliberate input stamps the time of the last user activity
// before any widget sees it, and the rest do not (#1108).
func TestWindow_InputStampsUserActivity(t *testing.T) {
	for name, tc := range map[string]struct {
		input  func(w *window)
		stamps bool
	}{
		"scroll":       {func(w *window) { w.processMouseScrolled(0, -3) }, true},
		"mouse press":  {func(w *window) { w.processMouseClicked(desktop.MouseButtonPrimary, press, 0) }, true},
		"key press":    {func(w *window) { w.press("A", 0) }, true},
		"typed rune":   {func(w *window) { w.processCharInput('a') }, true},
		"touch down":   {func(w *window) { w.handleTouch(1, touchBegin, geo.NewPos(10, 10)) }, true},
		"touch cancel": {func(w *window) { w.handleTouch(1, touchCancel, geo.NewPos(10, 10)) }, false},
		"drag": {func(w *window) {
			w.mouseButton = desktop.MouseButtonPrimary
			w.processMouseMoved(20, 20)
		}, true},
		"bare mouse move": {func(w *window) { w.processMouseMoved(20, 20) }, false},
	} {
		t.Run(name, func(t *testing.T) {
			w := keyWindow(widget.NewLabel("idle"))
			activity.Reset()
			t.Cleanup(activity.Reset)

			tc.input(w)

			if tc.stamps {
				assert.False(t, activity.Last().IsZero(), "%s counts as user activity", name)
			} else {
				assert.True(t, activity.Last().IsZero(), "%s is not user activity", name)
			}
		})
	}
}
