package metadata

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPXFRoundTrip exercises the full BelluiApp schema — including the optional
// nested messages, repeated fields and maps not present in the golden testdata —
// through a Save/Load cycle over the textual PXF format.
func TestPXFRoundTrip(t *testing.T) {
	in := &BelluiApp{
		Website:     "https://example.com",
		Description: "An example app",
		Details: AppDetails{
			Icon:    "icon.png",
			Name:    "Example",
			ID:      "com.example.app",
			Version: "2.3.4",
			Build:   7,
			FileAssociations: []FileAssociation{
				{Extensions: []string{"txt", "text"}, MimeType: "text/plain", Description: "Text", Role: "Editor", Icon: "doc.png"},
				{Extensions: []string{"md"}, MimeType: "text/markdown"},
			},
			URLSchemes: []string{"example", "example-alt"},
		},
		Development:  map[string]string{"Key": "dev"},
		Release:      map[string]string{"Key": "rel"},
		Languages:    []string{"en", "de"},
		Migrations:   map[string]bool{"belluiDo": true},
		AdaptiveIcon: &AdaptiveIcon{Foreground: "fg.png", Background: "bg.png", Monochrome: "mono.png"},
		Source:       &AppSource{Repo: "https://example.com/repo", Dir: "cmd/app"},
		CanOpen:      &CanOpen{MimeTypes: "text/plain;application/json"},
		LinuxAndBSD: &LinuxAndBSD{
			GenericName: "Editor",
			Categories:  []string{"Utility", "TextEditor"},
			Comment:     "Edits things",
			Keywords:    []string{"edit", "text"},
			ExecParams:  "%F",
		},
	}

	buf := &bytes.Buffer{}
	assert.NoError(t, Save(in, buf))

	out, err := Load(bytes.NewReader(buf.Bytes()))
	assert.NoError(t, err)

	assert.Equal(t, in, out)
}

// TestPXFEmpty confirms an empty manifest round-trips to a zero-value BelluiApp
// (proto3 omits zero values, and optional nested messages stay nil).
func TestPXFEmpty(t *testing.T) {
	buf := &bytes.Buffer{}
	assert.NoError(t, Save(&BelluiApp{}, buf))

	out, err := Load(bytes.NewReader(buf.Bytes()))
	assert.NoError(t, err)
	assert.Nil(t, out.AdaptiveIcon)
	assert.Nil(t, out.Source)
	assert.Nil(t, out.CanOpen)
	assert.Nil(t, out.LinuxAndBSD)
	assert.Empty(t, out.Website)
}
