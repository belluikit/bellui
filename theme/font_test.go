package theme_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"
)

func Test_DefaultSymbolFont(t *testing.T) {
	expect := "InterSymbols-Regular.ttf"
	result := theme.DefaultSymbolFont().Name()
	assert.Equal(t, expect, result, "wrong default text font")
}

func Test_DefaultTextBoldFont(t *testing.T) {
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	expect := "Roboto-Bold.ttf"
	result := theme.DefaultTextBoldFont().Name()
	assert.Equal(t, expect, result, "wrong default text bold font")
}

func Test_DefaultTextBoldItalicFont(t *testing.T) {
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	expect := "Roboto-BoldItalic.ttf"
	result := theme.DefaultTextBoldItalicFont().Name()
	assert.Equal(t, expect, result, "wrong default text bold italic font")
}

func Test_DefaultTextFont(t *testing.T) {
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	expect := "Roboto-Regular.ttf"
	result := theme.DefaultTextFont().Name()
	assert.Equal(t, expect, result, "wrong default text font")
}

func Test_DefaultTextItalicFont(t *testing.T) {
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	expect := "Roboto-Italic.ttf"
	result := theme.DefaultTextItalicFont().Name()
	assert.Equal(t, expect, result, "wrong default text italic font")
}

func Test_DefaultTextMonospaceFont(t *testing.T) {
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	expect := "OfficeCodePro-Regular.otf"
	result := theme.DefaultTextMonospaceFont().Name()
	assert.Equal(t, expect, result, "wrong default monospace font")
}

func Test_TextBoldFont(t *testing.T) {
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	expect := "Roboto-Bold.ttf"
	result := theme.TextBoldFont().Name()
	assert.Equal(t, expect, result, "wrong bold text font")
}

func Test_TextBoldItalicFont(t *testing.T) {
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	expect := "Roboto-BoldItalic.ttf"
	result := theme.TextBoldItalicFont().Name()
	assert.Equal(t, expect, result, "wrong bold italic text font")
}

func Test_TextFont(t *testing.T) {
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	expect := "Roboto-Regular.ttf"
	result := theme.TextFont().Name()
	assert.Equal(t, expect, result, "wrong regular text font")
}

func Test_TextItalicFont(t *testing.T) {
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	expect := "Roboto-Italic.ttf"
	result := theme.TextItalicFont().Name()
	assert.Equal(t, expect, result, "wrong italic text font")
}

func Test_TextMonospaceFont(t *testing.T) {
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	expect := "OfficeCodePro-Regular.otf"
	result := theme.TextMonospaceFont().Name()
	assert.Equal(t, expect, result, "wrong monospace font")
}

func Test_TextSymbolFont(t *testing.T) {
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	expect := "InterSymbols-Regular.ttf"
	result := theme.SymbolFont().Name()
	assert.Equal(t, expect, result, "wrong symbol font")
}
