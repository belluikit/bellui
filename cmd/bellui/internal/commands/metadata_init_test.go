package commands

import (
	"bytes"
	"go/format"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
)

// The metadata init file carries every AppMetadata field. Release, Custom and
// Migrations used to be emitted only for Fyne >= 2.3 / >= 2.6, versions no
// bellui release has (#337).
func TestMetadataInitTemplateCarriesAllFields(t *testing.T) {
	data := &appData{
		Name:           "Notes",
		AppID:          "com.example.notes",
		AppVersion:     "1.0.0",
		AppBuild:       3,
		ResGoString:    "nil",
		Release:        true,
		CustomMetadata: map[string]string{"channel": "beta"},
		Migrations:     map[string]bool{"belluiDo": true},
	}

	var buf bytes.Buffer
	require.NoError(t, templates.BelluiMetadataInit.Execute(&buf, data))

	src, err := format.Source(buf.Bytes())
	require.NoError(t, err, "the generated file must be valid Go:\n%s", buf.String())

	out := string(src)
	assert.Regexp(t, `ID:\s+"com.example.notes"`, out)
	assert.Regexp(t, `Release:\s+true`, out)
	assert.Contains(t, out, `"channel": "beta"`)
	assert.Contains(t, out, `"belluiDo": true`)
}
