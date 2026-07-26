package metadata

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFileAssociationsRoundTrip verifies the FileAssociations / URLSchemes schema
// additions serialize to TOML and parse back without loss.
func TestFileAssociationsRoundTrip(t *testing.T) {
	in := &BelluiApp{
		Details: AppDetails{
			Name:       "Testing",
			ID:         "com.example.testing",
			URLSchemes: []string{"testing", "testing-alt"},
			FileAssociations: []FileAssociation{
				{
					Extensions:  []string{"txt", "text"},
					MimeType:    "text/plain",
					Description: "Text Document",
					Role:        "Editor",
					Icon:        "doc.png",
				},
				{
					Extensions: []string{"md"},
					MimeType:   "text/markdown",
				},
			},
		},
	}

	buf := &bytes.Buffer{}
	assert.NoError(t, Save(in, buf))

	out, err := Load(bytes.NewReader(buf.Bytes()))
	assert.NoError(t, err)

	assert.Equal(t, []string{"testing", "testing-alt"}, out.Details.URLSchemes)
	assert.Len(t, out.Details.FileAssociations, 2)
	assert.Equal(t, in.Details.FileAssociations, out.Details.FileAssociations)
}
