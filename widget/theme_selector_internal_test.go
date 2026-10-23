package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	internalapp "github.com/belluikit/bellui/internal/app"
	internaltheme "github.com/belluikit/bellui/internal/theme"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

func TestThemeSelector_New(t *testing.T) {
	test.NewTempApp(t)

	sel := NewThemeSelector(nil)
	assert.Equal(t, ThemeVariantSystem, sel.Selection.Variant)
	assert.Equal(t, theme.ColorBlue, sel.Selection.Primary)

	r := test.TempWidgetRenderer(t, sel).(*themeSelectorRenderer)
	assert.Len(t, r.swatches, len(theme.PrimaryColorNames()))
	assert.Equal(t, 0, r.variants.Selected)
	min := sel.MinSize()
	assert.Positive(t, min.Width)
	assert.Positive(t, min.Height)
}

func TestThemeSelector_TapSwatch(t *testing.T) {
	test.NewTempApp(t)

	fired := 0
	var got ThemeSelection
	sel := NewThemeSelector(func(s ThemeSelection) { got = s; fired++ })
	r := test.TempWidgetRenderer(t, sel).(*themeSelectorRenderer)

	// pick a swatch that is not already selected
	target := r.swatches[0]
	if target.name == sel.Selection.Primary {
		target = r.swatches[1]
	}

	test.Tap(target)
	assert.Equal(t, 1, fired)
	assert.Equal(t, target.name, got.Primary)
	assert.Equal(t, target.name, sel.Selection.Primary)

	// tapping the already selected swatch is a no-op
	test.Tap(target)
	assert.Equal(t, 1, fired)
}

func TestThemeSelector_VariantChange(t *testing.T) {
	test.NewTempApp(t)

	fired := 0
	var got ThemeSelection
	sel := NewThemeSelector(func(s ThemeSelection) { got = s; fired++ })
	r := test.TempWidgetRenderer(t, sel).(*themeSelectorRenderer)

	// arrow right on the segmented button moves System -> Light
	r.variants.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, 1, fired)
	assert.Equal(t, ThemeVariantLight, got.Variant)
	assert.Equal(t, ThemeVariantLight, sel.Selection.Variant)
}

func TestThemeSelector_SetSelection(t *testing.T) {
	test.NewTempApp(t)

	fired := 0
	sel := NewThemeSelector(func(ThemeSelection) { fired++ })
	r := test.TempWidgetRenderer(t, sel).(*themeSelectorRenderer)

	want := ThemeSelection{Variant: ThemeVariantDark, Primary: theme.ColorRed}
	sel.SetSelection(want)
	assert.Equal(t, 1, fired)
	assert.Equal(t, want, sel.Selection)
	assert.Equal(t, int(ThemeVariantDark), r.variants.Selected)

	// setting the same selection again does not fire
	sel.SetSelection(want)
	assert.Equal(t, 1, fired)
}

func TestThemeSelection_Apply(t *testing.T) {
	a := test.NewTempApp(t)
	unpinned := a.Settings().ThemeVariant()

	ThemeSelection{Variant: ThemeVariantLight, Primary: theme.ColorGreen}.Apply()
	assert.Equal(t, theme.ColorGreen, a.Settings().PrimaryColor())
	assert.Equal(t, theme.VariantLight, a.Settings().ThemeVariant())

	ThemeSelection{Variant: ThemeVariantDark, Primary: theme.ColorRed}.Apply()
	assert.Equal(t, theme.ColorRed, a.Settings().PrimaryColor())
	assert.Equal(t, theme.VariantDark, a.Settings().ThemeVariant())

	// System withdraws the pin, so the test app reports no preference again,
	// as before any choice; empty Primary is untouched. The live OS follow
	// this buys is tested against the real settings in the app package.
	ThemeSelection{Variant: ThemeVariantSystem}.Apply()
	assert.Equal(t, theme.ColorRed, a.Settings().PrimaryColor())
	assert.Equal(t, unpinned, a.Settings().ThemeVariant())
}

// Settings that cannot withdraw a pin get the OS appearance at call time,
// pinned, rather than nothing (#695).
func TestThemeSelection_ApplySystemFallback(t *testing.T) {
	a := test.NewTempApp(t)
	bellui.SetCurrentApp(&plainSettingsApp{App: a, set: plainSettings{a.Settings()}})
	t.Cleanup(func() { bellui.SetCurrentApp(a) })

	ThemeSelection{Variant: ThemeVariantSystem}.Apply()
	assert.Equal(t, internalapp.DefaultVariant(), a.Settings().ThemeVariant())
}

// plainSettings hides the optional interfaces of the Settings it wraps.
type plainSettings struct{ bellui.Settings }

type plainSettingsApp struct {
	bellui.App
	set bellui.Settings
}

func (a *plainSettingsApp) Settings() bellui.Settings { return a.set }

func TestThemeSelection_SaveLoad(t *testing.T) {
	a := test.NewTempApp(t)
	p := a.Preferences()

	_, ok := LoadThemeSelection(p)
	assert.False(t, ok)

	for _, want := range []ThemeSelection{
		{Variant: ThemeVariantSystem, Primary: theme.ColorBlue},
		{Variant: ThemeVariantLight, Primary: theme.ColorPurple},
		{Variant: ThemeVariantDark, Primary: ""},
	} {
		SaveThemeSelection(want, p)
		got, ok := LoadThemeSelection(p)
		assert.True(t, ok)
		assert.Equal(t, want, got)
	}
}

func TestThemeSelector_SwatchRing(t *testing.T) {
	test.NewTempApp(t)

	sel := NewThemeSelector(nil)
	r := test.TempWidgetRenderer(t, sel).(*themeSelectorRenderer)

	for _, s := range r.swatches {
		sr := test.TempWidgetRenderer(t, s).(*themeSwatchRenderer)
		if s.name == sel.Selection.Primary {
			assert.Equal(t, internaltheme.PrimaryColorNamed(s.name), sr.ring.StrokeColor)
		} else {
			assert.NotEqual(t, internaltheme.PrimaryColorNamed(s.name), sr.ring.StrokeColor)
		}
	}
}
