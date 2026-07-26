package theme_test

import (
	"image/color"
	"io"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

func Test_DefaultTheme_AllColorsDefined(t *testing.T) {
	test.AssertAllColorNamesDefined(t, theme.DefaultTheme(), "default")
}

func Test_DefaultTheme_PrimaryForegroundColor(t *testing.T) {
	defaultTheme := theme.DefaultTheme()

	for _, name := range theme.PrimaryColorNames() {
		t.Run("primary foreground color "+name, func(t *testing.T) {
			oldApp := bellui.CurrentApp()
			defer bellui.SetCurrentApp(oldApp)
			bellui.SetCurrentApp(&themedApp{theme: defaultTheme, primaryColor: name})
			t.Run("light variant", func(t *testing.T) {
				c := defaultTheme.Color(theme.ColorNameForegroundOnPrimary, theme.VariantLight)
				assert.NotNil(t, c)
				assert.NotEqual(t, color.Transparent, c)
			})
			t.Run("dark variant", func(t *testing.T) {
				c := defaultTheme.Color(theme.ColorNameForegroundOnPrimary, theme.VariantDark)
				assert.NotNil(t, c)
				assert.NotEqual(t, color.Transparent, c)
			})
		})
	}
}

func Test_DefaultTheme_ShadowColor(t *testing.T) {
	t.Run("light", func(t *testing.T) {
		_, _, _, a := theme.DefaultTheme().Color(theme.ColorNameShadow, theme.VariantLight).RGBA()
		assert.NotEqual(t, 255, a, "should not be transparent")
	})

	t.Run("dark", func(t *testing.T) {
		_, _, _, a := theme.DefaultTheme().Color(theme.ColorNameShadow, theme.VariantDark).RGBA()
		assert.NotEqual(t, 255, a, "should not be transparent")
	})
}

func TestEmptyTheme(t *testing.T) {
	// Discarding log output for tests
	// The following functions tested all log an error when returning nil
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	bellui.CurrentApp().Settings().SetTheme(&emptyTheme{})
	assert.NotNil(t, theme.Color(theme.ColorNameForeground))
	assert.NotNil(t, theme.TextFont())
	assert.NotNil(t, theme.HelpIcon())
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
}

func TestThemeChange(t *testing.T) {
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	bg := theme.Color(theme.ColorNameBackground)

	bellui.CurrentApp().Settings().SetTheme(theme.LightTheme())
	assert.NotEqual(t, bg, theme.Color(theme.ColorNameBackground))
}

func TestTheme_Bootstrapping(t *testing.T) {
	current := bellui.CurrentApp().Settings().Theme()
	bellui.CurrentApp().Settings().SetTheme(nil)

	// this should not crash
	theme.Color(theme.ColorNameBackground)

	bellui.CurrentApp().Settings().SetTheme(current)
}

type emptyTheme struct{}

func (e *emptyTheme) Color(n bellui.ThemeColorName, v bellui.ThemeVariant) color.Color {
	return nil
}

func (e *emptyTheme) Font(s bellui.TextStyle) bellui.Resource {
	return nil
}

func (e *emptyTheme) Icon(n bellui.ThemeIconName) bellui.Resource {
	return nil
}

func (e *emptyTheme) Size(n bellui.ThemeSizeName) float32 {
	return 0
}
