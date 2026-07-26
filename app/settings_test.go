package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/belluikit/bellui"
	internalapp "github.com/belluikit/bellui/internal/app"
	"github.com/belluikit/bellui/internal/build"
	internalTest "github.com/belluikit/bellui/internal/test"
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

func TestSettingsLoad(t *testing.T) {
	settings := &settings{}

	err := settings.loadFromFile(filepath.Join("testdata", "light-theme.pxf"))
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "light", settings.schema.ThemeName)

	err = settings.loadFromFile(filepath.Join("testdata", "dark-theme.pxf"))
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "dark", settings.schema.ThemeName)
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

func TestOverrideTheme_IgnoresSettingsChange(t *testing.T) {
	// check that a file-load does not overwrite our value
	set := &settings{}
	err := os.Setenv("BELLUI_THEME", "light")
	if err != nil {
		t.Error(err)
	}
	set.setupTheme()
	assert.Equal(t, theme.DefaultTheme(), set.Theme())
	assert.Equal(t, theme.VariantLight, set.ThemeVariant())

	err = set.loadFromFile(filepath.Join("testdata", "dark-theme.pxf"))
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

func TestCustomTheme(t *testing.T) {
	type customTheme struct {
		bellui.Theme
	}
	set := &settings{}
	ctheme := &customTheme{internalTest.LightTheme(theme.DefaultTheme())}
	set.SetTheme(ctheme)

	set.setupTheme()
	assert.Equal(t, set.Theme(), ctheme)
	assert.Equal(t, internalapp.DefaultVariant(), set.ThemeVariant())

	err := set.loadFromFile(filepath.Join("testdata", "light-theme.pxf"))
	if err != nil {
		t.Error(err)
	}
	set.setupTheme()
	assert.Equal(t, set.Theme(), ctheme)
	assert.Equal(t, theme.VariantLight, set.ThemeVariant())

	err = set.loadFromFile(filepath.Join("testdata", "dark-theme.pxf"))
	if err != nil {
		t.Error(err)
	}
	set.setupTheme()
	assert.Equal(t, set.Theme(), ctheme)
	assert.Equal(t, theme.VariantDark, set.ThemeVariant())

	err = os.Setenv("BELLUI_THEME", "light")
	if err != nil {
		t.Error(err)
	}
	set.setupTheme()
	assert.Equal(t, set.Theme(), ctheme)
	assert.Equal(t, theme.VariantLight, set.ThemeVariant())

	err = os.Setenv("BELLUI_THEME", "dark")
	if err != nil {
		t.Error(err)
	}
	set.setupTheme()
	assert.Equal(t, set.Theme(), ctheme)
	assert.Equal(t, theme.VariantDark, set.ThemeVariant())

	err = os.Setenv("BELLUI_THEME", "")
	if err != nil {
		t.Error(err)
	}
	set.setupTheme()
	assert.Equal(t, set.Theme(), ctheme)
	assert.Equal(t, theme.VariantDark, set.ThemeVariant())
}
