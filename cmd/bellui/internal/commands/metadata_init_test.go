package commands

import (
	"bytes"
	"go/format"
	"os"
	"path/filepath"
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

// Every string in the init file is a Go literal built from user input (the
// manifest, --metadata), so quotes and backslashes must be escaped or the
// app stops compiling the moment its metadata is injected.
func TestMetadataInitTemplateEscapesStrings(t *testing.T) {
	data := &appData{
		Name:           `My "Pro" App`,
		AppID:          "com.example.notes",
		AppVersion:     "1.0.0",
		AppBuild:       1,
		ResGoString:    "nil",
		CustomMetadata: map[string]string{`dir\name`: `C:\work\x`},
		Migrations:     map[string]bool{`a"b`: true},
	}

	var buf bytes.Buffer
	require.NoError(t, templates.BelluiMetadataInit.Execute(&buf, data))

	src, err := format.Source(buf.Bytes())
	require.NoError(t, err, "the generated file must be valid Go:\n%s", buf.String())

	out := string(src)
	assert.Contains(t, out, `"My \"Pro\" App"`)
	assert.Contains(t, out, `"dir\\name": "C:\\work\\x"`)
	assert.Contains(t, out, `"a\"b": true`)
}

// Without a manifest or version flags, bellui build used to leave the
// runtime defaults alone; the injected file must carry the same defaults
// rather than Version "" and Build 0.
func TestCreateMetadataInitFileDefaultsVersionAndBuild(t *testing.T) {
	dir := t.TempDir()

	cleanup, err := createMetadataInitFile(dir, &appData{})
	require.NoError(t, err)
	t.Cleanup(cleanup)

	out, err := os.ReadFile(filepath.Join(dir, "bellui_metadata_init.go"))
	require.NoError(t, err)
	assert.Regexp(t, `Version:\s+"`+defaultAppVersion+`"`, string(out))
	assert.Regexp(t, `Build:\s+1,`, string(out))

	cleanup()
	assert.NoFileExists(t, filepath.Join(dir, "bellui_metadata_init.go"))
}

// A failure while writing the init file must not leave it behind: callers
// drop the cleanup on error and keep building, and an empty
// bellui_metadata_init.go breaks every later build of the app.
func TestCreateMetadataInitFileLeavesNothingOnError(t *testing.T) {
	dir := t.TempDir()

	_, err := createMetadataInitFile(dir, &appData{icon: filepath.Join(dir, "missing.png")})
	require.Error(t, err)
	assert.NoFileExists(t, filepath.Join(dir, "bellui_metadata_init.go"))
}
