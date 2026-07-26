package theme_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

func TestValidateTheme_Default(t *testing.T) {
	_ = test.NewTempApp(t)

	issues := theme.ValidateTheme(theme.DefaultTheme())
	for _, issue := range issues {
		t.Log(issue)
	}
}

func TestValidateTheme_Light(t *testing.T) {
	_ = test.NewTempApp(t)

	issues := theme.ValidateTheme(theme.LightTheme())
	for _, issue := range issues {
		t.Log(issue)
	}
}

// brokenTheme intentionally returns bad values.
type brokenTheme struct{}

func (b *brokenTheme) Color(bellui.ThemeColorName, bellui.ThemeVariant) color.Color { return nil }
func (b *brokenTheme) Font(bellui.TextStyle) bellui.Resource                        { return nil }
func (b *brokenTheme) Icon(bellui.ThemeIconName) bellui.Resource                    { return nil }
func (b *brokenTheme) Size(bellui.ThemeSizeName) float32                            { return 0 }

func TestValidateTheme_Broken(t *testing.T) {
	issues := theme.ValidateTheme(&brokenTheme{})
	if len(issues) == 0 {
		t.Error("expected issues from broken theme")
	}
	t.Logf("found %d issues (expected)", len(issues))
}
