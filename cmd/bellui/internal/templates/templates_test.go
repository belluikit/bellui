package templates

import (
	"bytes"
	"encoding/xml"
	"html"
	"image"
	"image/gif"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultIcon(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(DefaultIcon))
	require.NoError(t, err)
	assert.Equal(t, 1024, img.Bounds().Dx())
	assert.Equal(t, 1024, img.Bounds().Dy())
	// The icon sits on docks, taskbars and the dark splash page, so its
	// background must be clear. The logo's source file had faint noise there.
	assert.Zero(t, opaqueCornerPixels(img, 64), "corners must be fully transparent")
}

func TestSpinnersAreAnimated(t *testing.T) {
	for name, data := range map[string][]byte{"dark": SpinnerDark, "light": SpinnerLight} {
		g, err := gif.DecodeAll(bytes.NewReader(data))
		require.NoError(t, err, name)
		assert.Greater(t, len(g.Image), 1, name)
		assert.Equal(t, 0, g.LoopCount, name) // loop forever
		// One GIF serves both themes only while its background is clear.
		assert.Zero(t, opaqueCornerPixels(g.Image[0], 8), name+": corners must be transparent")
	}
}

// opaqueCornerPixels counts the pixels that are not fully transparent in the
// four s×s corners of img.
func opaqueCornerPixels(img image.Image, s int) int {
	b := img.Bounds()
	n := 0
	for _, o := range []image.Point{{b.Min.X, b.Min.Y}, {b.Max.X - s, b.Min.Y}, {b.Min.X, b.Max.Y - s}, {b.Max.X - s, b.Max.Y - s}} {
		for y := o.Y; y < o.Y+s; y++ {
			for x := o.X; x < o.X+s; x++ {
				if _, _, _, a := img.At(x, y).RGBA(); a != 0 {
					n++
				}
			}
		}
	}
	return n
}

// The splash page uses the app's font where the device has it, else the
// system UI font. It must not ship or fetch a font of its own.
func TestIndexHTMLUsesSystemFonts(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, IndexHTML.Execute(&out, struct {
		AppName, AppVersion, WasmFile string
		IsReleased, WebGPU            bool
	}{"App", "1.0.0", "App.wasm", true, true}))
	page := out.String()
	assert.Contains(t, page, "font-family: Roboto, system-ui,")
	assert.NotContains(t, page, "@font-face")
	assert.NotContains(t, page, "fonts.googleapis.com")
}

// The web page must declare a viewport, or phone browsers lay it out 980 CSS px
// wide: the app shows as a shrunken desktop page and its canvas gets about six
// times the pixels it needs.
func TestIndexHTMLHasViewport(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, IndexHTML.Execute(&out, struct {
		AppName, AppVersion, WasmFile string
		IsReleased, WebGPU            bool
	}{"App", "1.0.0", "App.wasm", true, true}))
	assert.Contains(t, out.String(), `<meta name="viewport" content="width=device-width, initial-scale=1`)
}

// The templates put GopherCredit into XML (Info.plist, AndroidManifest.xml)
// without escaping it, so it must not need any.
func TestGopherCreditNeedsNoEscaping(t *testing.T) {
	assert.Equal(t, html.EscapeString(GopherCredit), GopherCredit)
}

// TestAndroidManifestEscapesText checks that manifest text containing XML
// metacharacters still renders a well-formed AndroidManifest.xml.
func TestAndroidManifestEscapesText(t *testing.T) {
	var b strings.Builder
	data := struct {
		JavaPkgPath, Name, LibName, Version, Credit string
		Debug, AdaptiveIcon                         bool
		Build                                       int
		URLSchemes, MimeTypes                       []string
	}{
		JavaPkgPath: "com.example.app", Name: `Tom & Jerry's <"Cat">`, LibName: "app",
		Version: "1.0", Credit: "a & b", Build: 1,
		URLSchemes: []string{"x&y"}, MimeTypes: []string{"text/<plain>"},
	}
	if err := ManifestAndroid.Execute(&b, data); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		App struct {
			Label string `xml:"http://schemas.android.com/apk/res/android label,attr"`
		} `xml:"application"`
	}
	if err := xml.Unmarshal([]byte(b.String()), &doc); err != nil {
		t.Fatalf("manifest is not well-formed: %v\n%s", err, b.String())
	}
	if doc.App.Label != data.Name {
		t.Errorf("label = %q, want %q", doc.App.Label, data.Name)
	}
}
