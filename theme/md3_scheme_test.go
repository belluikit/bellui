package theme

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDarkSchemeFromPalettes(t *testing.T) {
	p := CorePalettesFromPrimary(color.NRGBA{R: 0x67, G: 0x50, B: 0xA4, A: 0xff})
	s := DarkSchemeFromPalettes(p)

	assert.NotNil(t, s)
	assert.NotEqual(t, color.NRGBA{}, s.Primary, "Primary should be non-zero")
	assert.NotEqual(t, color.NRGBA{}, s.OnPrimary, "OnPrimary should be non-zero")
	assert.NotEqual(t, color.NRGBA{}, s.Surface, "Surface should be non-zero")
	assert.NotEqual(t, color.NRGBA{}, s.Error, "Error should be non-zero")
	assert.Equal(t, uint8(0xff), s.Shadow.A, "Shadow should be fully opaque")
	assert.Equal(t, uint8(0x80), s.Scrim.A, "Scrim should be semi-transparent")
}

func TestLightSchemeFromPalettes(t *testing.T) {
	p := CorePalettesFromPrimary(color.NRGBA{R: 0x67, G: 0x50, B: 0xA4, A: 0xff})
	s := LightSchemeFromPalettes(p)

	assert.NotNil(t, s)
	assert.NotEqual(t, color.NRGBA{}, s.Primary, "Primary should be non-zero")
	assert.NotEqual(t, color.NRGBA{}, s.OnPrimary, "OnPrimary should be non-zero")
	assert.NotEqual(t, color.NRGBA{}, s.Surface, "Surface should be non-zero")
	assert.NotEqual(t, color.NRGBA{}, s.Error, "Error should be non-zero")
}

func TestDarkAndLightSchemesDiffer(t *testing.T) {
	p := CorePalettesFromPrimary(color.NRGBA{R: 0x29, G: 0x6F, B: 0xF6, A: 0xff})
	dark := DarkSchemeFromPalettes(p)
	light := LightSchemeFromPalettes(p)

	assert.NotEqual(t, dark.Primary, light.Primary, "Dark and light Primary should differ")
	assert.NotEqual(t, dark.Surface, light.Surface, "Dark and light Surface should differ")
}

func TestMd3SchemeColor_KnownName(t *testing.T) {
	p := CorePalettesFromPrimary(color.NRGBA{R: 0x67, G: 0x50, B: 0xA4, A: 0xff})
	s := DarkSchemeFromPalettes(p)

	c := md3SchemeColor(s, ColorNameSecondaryContainer)
	assert.NotNil(t, c)
	assert.Equal(t, s.SecondaryContainer, *c)
}

func TestMd3SchemeColor_UnknownName(t *testing.T) {
	p := CorePalettesFromPrimary(color.NRGBA{R: 0x67, G: 0x50, B: 0xA4, A: 0xff})
	s := DarkSchemeFromPalettes(p)

	c := md3SchemeColor(s, "nonExistentColorName")
	assert.Nil(t, c)
}
