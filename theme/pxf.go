package theme

import (
	"io"
	"strings"

	"github.com/trendvidia/protowire-go/encoding/pxf"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/theme/themepb"
)

// FromPXF returns a Theme created from the given PXF data.
// Any values not present in the data will fall back to the default theme.
// If a parse error occurs it will be returned along with a default theme.
//
// The data names colors, sizes, fonts and icons by their theme names:
//
//	colors = { foreground: "#ffffff" }
//	colors_light = { background: "#303030" }
//	sizes = { iconInline: 10 }
//	fonts = { monospace: "file:///path/to/font.ttf" }
//
// colors apply to both variants unless colors_dark or colors_light names the
// same color.
//
// Since: 1.3
func FromPXF(data string) (bellui.Theme, error) {
	return FromPXFReader(strings.NewReader(data))
}

// FromPXFWithFallback returns a Theme created from the given PXF data.
// Any values not present in the data will fall back to the specified theme.
// If a parse error occurs it will be returned along with a specified fallback theme.
//
// Since: 1.3
func FromPXFWithFallback(data string, fallback bellui.Theme) (bellui.Theme, error) {
	return fromPXFWithFallback(strings.NewReader(data), fallback)
}

// FromPXFReader returns a Theme created from the given PXF data through the reader.
// Any values not present in the data will fall back to the default theme.
// If a parse error occurs it will be returned along with a default theme.
//
// Since: 1.3
func FromPXFReader(r io.Reader) (bellui.Theme, error) {
	return fromPXFWithFallback(r, DefaultTheme())
}

// FromPXFReaderWithFallback returns a Theme created from the given PXF data through the reader.
// Any values not present in the data will fall back to the specified theme.
// If a parse error occurs it will be returned along with a specified fallback theme.
//
// Since: 1.3
func FromPXFReaderWithFallback(r io.Reader, fallback bellui.Theme) (bellui.Theme, error) {
	return fromPXFWithFallback(r, fallback)
}

func fromPXFWithFallback(r io.Reader, fallback bellui.Theme) (bellui.Theme, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return fallback, err
	}
	var msg themepb.Theme
	if err := pxf.Unmarshal(data, &msg); err != nil {
		return fallback, err
	}

	th := &schema{Sizes: msg.GetSizes()}
	for _, c := range []struct {
		from map[string]string
		to   *map[string]hexColor
	}{
		{msg.GetColors(), &th.Colors},
		{msg.GetColorsDark(), &th.DarkColors},
		{msg.GetColorsLight(), &th.LightColors},
	} {
		if len(c.from) == 0 {
			continue
		}
		*c.to = make(map[string]hexColor, len(c.from))
		for name, hex := range c.from {
			var col hexColor
			if err := col.parseColor(hex); err != nil {
				return fallback, err
			}
			(*c.to)[name] = col
		}
	}
	for _, u := range []struct {
		from map[string]string
		to   *map[string]uriString
	}{
		{msg.GetFonts(), &th.Fonts},
		{msg.GetIcons(), &th.Icons},
	} {
		if len(u.from) == 0 {
			continue
		}
		*u.to = make(map[string]uriString, len(u.from))
		for name, uri := range u.from {
			(*u.to)[name] = uriString(uri)
		}
	}

	return &dataTheme{data: th, fallback: fallback}, nil
}
