//go:build (!windows || !ci) && !wasm

package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/activity"
	"github.com/belluikit/bellui/internal/driver/mobile/event/key"
	"github.com/belluikit/bellui/internal/driver/mobile/event/textedit"
	"github.com/belluikit/bellui/internal/driver/mobile/event/touch"
	"github.com/belluikit/bellui/internal/textinput"
	"github.com/belluikit/bellui/widget"
)

// Every kind of deliberate input stamps the time of the last user activity
// before any widget sees it, and a touch the system cancels does not (#1108).
// A finger moving with no tap, which is how a list is scrolled, counts on its
// own: that is the case an app could not see.
func Test_driver_InputStampsUserActivity(t *testing.T) {
	d := &driver{textInput: textinput.NewSession(keyboardStateBackend{})}
	w, _ := buttonWindow(t, d, "activity")
	closeOnCleanup(t, w)
	m := middle(w)
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(widget.NewEntry())
	t.Cleanup(activity.Reset)

	stamped := func(name string, input func(), want bool) {
		t.Helper()
		activity.Reset()
		input()
		assert.Equal(t, want, !activity.Last().IsZero(), "%s stamps user activity: %v", name, want)
	}

	stamped("touch down", func() {
		d.routeTouch(w, touch.Event{X: m, Y: m, Sequence: 0, Type: touch.TypeBegin}, false)
	}, true)
	stamped("touch move", func() {
		d.routeTouch(w, touch.Event{X: m, Y: m + 40, Sequence: 0, Type: touch.TypeMove}, false)
	}, true)
	stamped("touch cancel", func() {
		d.routeTouch(w, touch.Event{X: m, Y: m + 40, Sequence: 0, Type: touch.TypeCancel}, false)
	}, false)
	stamped("explore by touch", func() {
		d.routeTouch(w, touch.Event{X: m, Y: m, Sequence: 1, Type: touch.TypeBegin}, true)
	}, true)
	stamped("key", func() {
		d.dispatchKeyEvent(c, key.Event{Code: key.CodeA, Rune: 'a', Direction: key.DirPress})
	}, true)
	stamped("soft keyboard edit", func() {
		d.handleTextEdit(c, textedit.Event{Start: 0, End: 0, Text: "a"})
	}, true)
}
