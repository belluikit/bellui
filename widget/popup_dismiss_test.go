package widget

import (
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
)

// popUpDismissPaths are the user-initiated ways a pop-up is dismissed. Each one
// runs OnDismissed when it is set (#693).
var popUpDismissPaths = []struct {
	name    string
	dismiss func(*PopUp)
}{
	{"Escape", func(p *PopUp) { p.DismissOnEscape() }},
	{"OutsideTap", func(p *PopUp) { test.Tap(p) }},
	{"SecondaryTap", func(p *PopUp) { test.TapSecondary(p) }},
	{"AccessibleDismiss", func(p *PopUp) { p.PerformAccessibleAction(core.ActionDismiss) }},
}

func TestPopUp_OnDismissed_RunsOnEveryUserDismissal(t *testing.T) {
	test.NewTempApp(t)
	for _, path := range popUpDismissPaths {
		t.Run(path.name, func(t *testing.T) {
			pop := NewPopUp(NewLabel("Hi"), test.Canvas())
			defer test.Canvas().Overlays().Remove(pop)
			calls := 0
			pop.OnDismissed = func() { calls++ }
			pop.Show()

			path.dismiss(pop)

			assert.Equal(t, 1, calls, "OnDismissed must run on %s", path.name)
			// OnDismissed replaces hiding: the host owns the close path.
			assert.True(t, pop.Visible(), "pop-up must not be auto-hidden when OnDismissed is set")
			assert.Contains(t, test.Canvas().Overlays().List(), core.CanvasObject(pop))
		})
	}
}

// A modal pop-up that dismisses on an outside tap (a modal NavigationDrawer)
// takes the same route as a plain one.
func TestPopUp_OnDismissed_ModalDismissOnTap(t *testing.T) {
	test.NewTempApp(t)
	for _, path := range popUpDismissPaths {
		t.Run(path.name, func(t *testing.T) {
			pop := NewModalPopUp(NewLabel("Hi"), test.Canvas())
			pop.modalDismissOnTap = true
			defer test.Canvas().Overlays().Remove(pop)
			calls := 0
			pop.OnDismissed = func() { calls++ }
			pop.Show()

			path.dismiss(pop)

			assert.Equal(t, 1, calls)
			assert.True(t, pop.Visible())
		})
	}
}

func TestPopUp_OnDismissed_NilHides(t *testing.T) {
	test.NewTempApp(t)
	for _, path := range popUpDismissPaths {
		t.Run(path.name, func(t *testing.T) {
			pop := NewPopUp(NewLabel("Hi"), test.Canvas())
			defer test.Canvas().Overlays().Remove(pop)
			pop.Show()

			path.dismiss(pop)

			assert.False(t, pop.Visible())
			assert.NotContains(t, test.Canvas().Overlays().List(), core.CanvasObject(pop))
		})
	}
}

func TestPopUp_OnDismissed_NotRunByHide(t *testing.T) {
	test.NewTempApp(t)
	pop := NewPopUp(NewLabel("Hi"), test.Canvas())
	defer test.Canvas().Overlays().Remove(pop)
	pop.OnDismissed = func() { t.Error("Hide from code must not run OnDismissed") }
	pop.Show()

	pop.Hide()

	assert.False(t, pop.Visible())
}

// A modal pop-up that does not dismiss on tap (every canvas dialog) ignores an
// outside tap, a secondary tap and the accessibility dismiss action, so its
// OnDismissed stays Escape-only in practice.
func TestPopUp_OnDismissed_ModalIgnoresTapAndAccessibleDismiss(t *testing.T) {
	test.NewTempApp(t)
	for _, path := range popUpDismissPaths[1:] {
		t.Run(path.name, func(t *testing.T) {
			pop := NewModalPopUp(NewLabel("Hi"), test.Canvas())
			defer test.Canvas().Overlays().Remove(pop)
			pop.OnDismissed = func() { t.Errorf("%s must not dismiss a modal pop-up", path.name) }
			pop.Show()

			path.dismiss(pop)

			assert.True(t, pop.Visible())
		})
	}
}
