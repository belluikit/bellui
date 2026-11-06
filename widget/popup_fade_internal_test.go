package widget

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// heldAnimationsApp is the test app with animations that do not run: the test
// driver ends each animation as it starts, so a fade could never be caught
// half-way. The test ends one by hand with Tick.
type heldAnimationsApp struct {
	bellui.App
	driver heldAnimationsDriver
}

func (a *heldAnimationsApp) Driver() bellui.Driver { return &a.driver }

type heldAnimationsDriver struct{ bellui.Driver }

func (*heldAnimationsDriver) StartAnimation(*bellui.Animation) {}

func (*heldAnimationsDriver) StopAnimation(*bellui.Animation) {}

func holdAnimations(t *testing.T) core.Canvas {
	app := test.NewTempApp(t)
	held := &heldAnimationsApp{App: app, driver: heldAnimationsDriver{app.Driver()}}
	bellui.SetCurrentApp(held)
	t.Cleanup(func() { bellui.SetCurrentApp(app) })
	require.True(t, app.Settings().ShowAnimations(), "pop-ups fade only with animations on")
	w := app.NewWindow("Test")
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 300))
	return w.Canvas()
}

// TestPopUp_FadeEndKeepsOverlaysAddedDuringIt: when a pop-up's fade-out ends,
// it takes with it the overlays that were above it when it began hiding (a
// menu it opened), but not one added during the fade. OverlayStack.Remove
// drops everything above the overlay removed, so a pop-up shown in those
// ~100 ms vanished with the fading one (#1038, belluikit/player#342).
func TestPopUp_FadeEndKeepsOverlaysAddedDuringIt(t *testing.T) {
	c := holdAnimations(t)
	dialog := NewModalPopUp(NewLabel("Dialog"), c)
	dialog.Show()
	menu := NewPopUp(NewLabel("Opened by the dialog"), c)
	menu.Show()

	dialog.Hide()
	require.NotNil(t, dialog.hideAnim)
	later := NewPopUp(NewLabel("Shown during the fade"), c)
	later.Show()
	require.Equal(t, []core.CanvasObject{dialog, menu, later}, c.Overlays().List())

	dialog.hideAnim.Tick(1) // the fade ends
	assert.Equal(t, []core.CanvasObject{later}, c.Overlays().List())
}

// TestPopUp_PassThroughFadeRemovesOnlyItself: a toast or snackbar opens
// nothing, so its fade ending removes it alone, also a dialog opened above it
// while it showed (#1038).
func TestPopUp_PassThroughFadeRemovesOnlyItself(t *testing.T) {
	c := holdAnimations(t)
	s := NewSnackbar("Saved", "", nil)
	s.Duration = time.Minute
	s.Show(c)
	snack := s.popUp
	dialog := NewModalPopUp(NewLabel("Opened while it showed"), c)
	dialog.Show()

	s.Hide()
	later := NewPopUp(NewLabel("Shown during the fade"), c)
	later.Show()

	snack.hideAnim.Tick(1)
	assert.Equal(t, []core.CanvasObject{dialog, later}, c.Overlays().List())
}
