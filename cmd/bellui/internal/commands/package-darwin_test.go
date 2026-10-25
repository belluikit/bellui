package commands

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"github.com/belluikit/bellui/cmd/bellui/internal/metadata"
	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
	"github.com/stretchr/testify/assert"
)

func TestDarwinLangs(t *testing.T) {
	langs := darwinLangs([]string{"en-GB", "de-CH"})

	assert.Equal(t, 2, len(langs))
	assert.Equal(t, "en_GB", langs[0])
	assert.Equal(t, "de_CH", langs[1])
}

func TestDarwinFileAssociations(t *testing.T) {
	assocs := darwinFileAssociations([]metadata.FileAssociation{
		{Extensions: []string{"txt"}, MimeType: "text/plain", Description: "Text Document"},
		{Extensions: []string{"md"}}, // no description/role -> defaults
	})

	assert.Len(t, assocs, 2)
	assert.Equal(t, "Text Document", assocs[0].Name)
	assert.Equal(t, "Editor", assocs[0].Role)
	assert.Equal(t, []string{"text/plain"}, assocs[0].MimeTypes)

	// falls back to the first extension as the type name, Editor role, no mime types
	assert.Equal(t, "md", assocs[1].Name)
	assert.Equal(t, "Editor", assocs[1].Role)
	assert.Nil(t, assocs[1].MimeTypes)

	assert.Nil(t, darwinFileAssociations(nil))
}

func TestInfoPlistFileHandlers(t *testing.T) {
	tplData := darwinData{
		Name:  "Testing",
		AppID: "com.example.testing",
		FileAssociations: darwinFileAssociations([]metadata.FileAssociation{
			{Extensions: []string{"txt", "text"}, MimeType: "text/plain", Description: "Text Document"},
		}),
		URLSchemes: []string{"testing", "testing-alt"},
	}
	buf := &bytes.Buffer{}
	assert.NoError(t, templates.InfoPlistDarwin.Execute(buf, tplData))
	out := buf.String()

	assert.Contains(t, out, "<key>CFBundleDocumentTypes</key>")
	assert.Contains(t, out, "<string>Text Document</string>")
	assert.Contains(t, out, "<string>text/plain</string>")
	assert.Contains(t, out, "<string>text</string>")
	assert.Contains(t, out, "<key>CFBundleURLTypes</key>")
	assert.Contains(t, out, "<string>testing-alt</string>")
	assert.Contains(t, out, "<string>com.example.testing</string>")

	// no associations => neither block is emitted
	empty := &bytes.Buffer{}
	assert.NoError(t, templates.InfoPlistDarwin.Execute(empty, darwinData{Name: "Bare"}))
	assert.False(t, strings.Contains(empty.String(), "CFBundleDocumentTypes"))
	assert.False(t, strings.Contains(empty.String(), "CFBundleURLTypes"))
}

func TestDarwinUsageDescriptions(t *testing.T) {
	// Sorted by key for deterministic output; a map with two entries round-trips.
	entries := darwinUsageDescriptions(map[string]string{
		"NSMicrophoneUsageDescription":        "Record audio.",
		"NSLocationWhenInUseUsageDescription": "Report GPS position.",
	})
	assert.Len(t, entries, 2)
	assert.Equal(t, "NSLocationWhenInUseUsageDescription", entries[0].Key) // sorted
	assert.Equal(t, "NSMicrophoneUsageDescription", entries[1].Key)
	assert.Nil(t, darwinUsageDescriptions(nil))
}

func TestInfoPlistUsageDescriptions(t *testing.T) {
	// The darwin bundle reuses the app's IOSUsageDescriptions (same NS* keys), so
	// a packaged macOS app that uses GPS gets the system location prompt and real
	// fixes (pre-rebrand appviewer tracker #157).
	tplData := darwinData{
		Name:  "Testing",
		AppID: "com.example.testing",
		UsageDescriptions: darwinUsageDescriptions(map[string]string{
			"NSLocationWhenInUseUsageDescription": "Report GPS position, speed, and bearing.",
		}),
	}
	buf := &bytes.Buffer{}
	assert.NoError(t, templates.InfoPlistDarwin.Execute(buf, tplData))
	out := buf.String()
	assert.Contains(t, out, "<key>NSLocationWhenInUseUsageDescription</key>")
	assert.Contains(t, out, "<string>Report GPS position, speed, and bearing.</string>")

	// no usage descriptions => no NS* keys leak into the plist
	empty := &bytes.Buffer{}
	assert.NoError(t, templates.InfoPlistDarwin.Execute(empty, darwinData{Name: "Bare"}))
	assert.False(t, strings.Contains(empty.String(), "UsageDescription"))
}

// Manifest text reaches the plist escaped: a raw & or < made it unparseable.
func TestInfoPlistDarwinEscapesValues(t *testing.T) {
	buf := &bytes.Buffer{}
	assert.NoError(t, templates.InfoPlistDarwin.Execute(buf, darwinData{
		Name:              "Tom & Jerry",
		UsageDescriptions: []darwinUsageDescription{{Key: "NSCameraUsageDescription", Value: "Scan <docs>"}},
		FileAssociations: darwinFileAssociations([]metadata.FileAssociation{
			{Extensions: []string{"tj"}, Description: "Cat & Mouse"},
		}),
	}))
	d := xml.NewDecoder(bytes.NewReader(buf.Bytes()))
	for {
		_, err := d.Token()
		if err == io.EOF {
			break
		}
		if !assert.NoError(t, err, buf.String()) {
			return
		}
	}
	assert.Contains(t, buf.String(), "<string>Tom &amp; Jerry</string>")
	assert.Contains(t, buf.String(), "<string>Scan &lt;docs&gt;</string>")
	assert.Contains(t, buf.String(), "<string>Cat &amp; Mouse</string>")
}

// An app with no category gets no LSApplicationCategoryType: the key used to
// be written unconditionally as the invalid "public.app-category." (#361).
func TestInfoPlistDarwinCategory(t *testing.T) {
	render := func(category string) string {
		buf := &bytes.Buffer{}
		assert.NoError(t, templates.InfoPlistDarwin.Execute(buf, darwinData{Name: "Testing", Category: category}))
		return buf.String()
	}

	none := render("")
	assert.NotContains(t, none, "LSApplicationCategoryType")
	assert.NotContains(t, none, "public.app-category.")
	assert.Contains(t, none, "<key>LSMinimumSystemVersion</key>", "the keys around it are still written")

	// The template is checked out with CRLF line endings on Windows.
	withCategory := strings.ReplaceAll(render("utilities"), "\r\n", "\n")
	assert.Contains(t, withCategory,
		"<key>LSApplicationCategoryType</key>\n\t<string>public.app-category.utilities</string>")
}
