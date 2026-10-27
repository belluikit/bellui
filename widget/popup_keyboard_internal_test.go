package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// A dialog moves up above an on-screen keyboard, which a phone canvas reports
// as a taller bottom inset, and back when the keyboard closes. It stayed where
// it was, its buttons under the keyboard its own field had opened (#654).
func TestModalPopUp_StaysAboveTheKeyboard(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewTempWindow(t, NewLabel("content"))
	w.Resize(geo.NewSize(400, 600))
	c := &insetCanvas{Canvas: w.Canvas(), top: 24, bottom: 16}

	pop := NewModalPopUp(NewLabel("dialog"), c)
	pop.Resize(geo.NewSize(300, 200))
	pop.Show()
	defer pop.Hide()
	bg := func() (top, bottom float32) {
		r := test.TempWidgetRenderer(t, pop).(*modalPopUpRenderer)
		return r.background.Position().Y, r.background.Position().Y + r.background.Size().Height
	}
	centred := pop.Content.Position()
	if top, bottom := bg(); top != 200 || bottom != 400 {
		t.Fatalf("without a keyboard the dialog spans %v-%v, want it centred at 200-400", top, bottom)
	}

	c.bottom = 16 + 300 // the keyboard opens
	pop.Refresh()       // what the mobile canvas does when an inset changes
	areaPos, area := c.InteractiveArea()
	top, bottom := bg()
	assert.LessOrEqual(t, bottom, areaPos.Y+area.Height, "the dialog's bottom edge, its buttons, is under the keyboard")
	assert.GreaterOrEqual(t, top, areaPos.Y, "the dialog moved up under the status bar")

	c.bottom = 16 // the keyboard closes
	pop.Refresh()
	assert.Equal(t, centred, pop.Content.Position(), "the dialog did not return to the centre")
}
