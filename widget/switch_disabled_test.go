package widget_test

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// A switch on and off, enabled and disabled, in both variants of the default
// theme. Disabled and on, it draws M3's on-surface track at 12% and surface
// handle; the handle was on-surface, near black in the light theme (#1394).
func TestSwitch_DisabledLook(t *testing.T) {
	for _, v := range []struct {
		name    string
		variant bellui.ThemeVariant
	}{{"light", theme.VariantLight}, {"dark", theme.VariantDark}} {
		t.Run(v.name, func(t *testing.T) {
			a := test.NewTempApp(t)
			test.ApplyTheme(t, theme.DefaultTheme())
			a.Settings().(interface{ SetThemeVariant(bellui.ThemeVariant) }).SetThemeVariant(v.variant)

			sw := func(on, disabled bool) *widget.Switch {
				s := widget.NewSwitch(nil)
				s.On = on
				if disabled {
					s.Disable()
				}
				return s
			}
			w := test.NewTempWindow(t, container.NewGridWithColumns(4,
				sw(true, false), sw(false, false), sw(true, true), sw(false, true)))
			w.Resize(geo.NewSize(320, 60))
			test.AssertRendersToImage(t, "switch/disabled_look_"+v.name+".png", w.Canvas())
		})
	}
}
