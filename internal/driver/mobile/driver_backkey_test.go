//go:build !windows || !ci

package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver/mobile/event/key"
	"github.com/belluikit/bellui/widget"
)

// Android's Back button carries the Escape meaning (#259): it backs out of
// the innermost dismissible surface — the canvas file dialog's selection
// mode, then the dialog itself — and only with nothing left to dismiss does
// it fall through to the app's KeyBack delivery / GoBack.
func Test_driver_BackDismissesInnermostOverlay(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(widget.NewLabel("content"))
	c.Resize(geo.NewSize(200, 200))

	dismissed := 0
	pop := widget.NewModalPopUp(widget.NewLabel("dialog"), c)
	pop.OnDismissed = func() {
		dismissed++
		pop.Hide()
	}
	pop.Show()

	d.typeDownCanvas(c, 0, key.CodeBackButton, 0)
	assert.Equal(t, 1, dismissed, "Back dismisses the innermost overlay like Escape")

	// With nothing left to dismiss, Back reaches the app's key hook unchanged.
	var seen []event.KeyName
	c.SetOnTypedKey(func(ev *event.KeyEvent) {
		seen = append(seen, ev.Name)
	})
	d.typeDownCanvas(c, 0, key.CodeBackButton, 0)
	assert.Equal(t, []event.KeyName{mobile.KeyBack}, seen, "with no overlay Back falls through to the app")
	assert.Equal(t, 1, dismissed)
}
