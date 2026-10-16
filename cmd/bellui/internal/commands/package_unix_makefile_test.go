package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
)

// An app without an ID gets a .desktop file named after the app; the Makefile
// must install that same file rather than "$(AppID).desktop" with an empty ID
// (#337).
func TestMakefileInstallsTheDesktopFileThatWasWritten(t *testing.T) {
	desktop := unixData{Name: "Notes", AppID: "", Exec: "notes", Icon: "Notes"}

	data := makefileData(desktop, "Notes")
	assert.Equal(t, "Notes", data.AppID)
	assert.Equal(t, "Notes.png", data.Icon)

	var buf bytes.Buffer
	require.NoError(t, templates.MakefileUNIX.Execute(&buf, data))
	assert.Contains(t, buf.String(), `AppID := "Notes"`)
	assert.True(t, strings.Contains(buf.String(), "share/applications/$(AppID).desktop"))

	withID := makefileData(unixData{Name: "Notes", AppID: "com.example.notes", Icon: "com.example.notes"}, "com.example.notes")
	assert.Equal(t, "com.example.notes", withID.AppID)
}
