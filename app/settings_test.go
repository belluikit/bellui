package app

import (
	"os"
	"testing"

	"github.com/belluikit/bellui"
	internalapp "github.com/belluikit/bellui/internal/app"
	"github.com/belluikit/bellui/internal/build"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
)

func TestSettingsBuildType(t *testing.T) {
	set := test.NewApp().Settings()
	assert.Equal(t, bellui.BuildStandard, set.BuildType()) // during test we should have a normal build

	set = &settings{}
	assert.Equal(t, build.Mode, set.BuildType()) // when testing this package only it could be debug or release
}

func TestOverrideTheme(t *testing.T) {
	set := &settings{}
	set.setupTheme()
	assert.Equal(t, internalapp.DefaultVariant(), set.ThemeVariant())

	set.schema.ThemeName = "light"
	set.setupTheme()
	assert.Equal(t, theme.DefaultTheme(), set.Theme())
	assert.Equal(t, theme.VariantLight, set.ThemeVariant())

	set.schema.ThemeName = "dark"
	set.setupTheme()
	assert.Equal(t, theme.DefaultTheme(), set.Theme())
	assert.Equal(t, theme.VariantDark, set.ThemeVariant())

	set = &settings{}
	set.setupTheme()
	assert.Equal(t, internalapp.DefaultVariant(), set.ThemeVariant())

	err := os.Setenv("BELLUI_THEME", "light")
	if err != nil {
		t.Error(err)
	}
	set.setupTheme()
	assert.Equal(t, theme.DefaultTheme(), set.Theme())
	assert.Equal(t, theme.VariantLight, set.ThemeVariant())
	err = os.Setenv("BELLUI_THEME", "")
	if err != nil {
		t.Error(err)
	}
}
