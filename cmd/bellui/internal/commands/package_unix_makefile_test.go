package commands

import (
	"bytes"
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

	var buf bytes.Buffer
	require.NoError(t, templates.MakefileUNIX.Execute(&buf, makefileData(desktop, "Notes", "notes", "Notes.png")))
	assert.Contains(t, buf.String(), `AppID := "Notes"`)
	assert.Contains(t, buf.String(), "share/applications/$(AppID).desktop")
}

// The desktop entry's Exec carries a field code when the app opens files or
// URLs, and the icon keeps its source extension; the Makefile installs files,
// so it must name the binary and the icon packageUNIX actually wrote.
func TestMakefileInstallsTheBinaryAndIconThatWereWritten(t *testing.T) {
	desktop := unixData{Name: "Notes", AppID: "com.example.notes", Exec: "notes %U", Icon: "com.example.notes"}

	var buf bytes.Buffer
	require.NoError(t, templates.MakefileUNIX.Execute(&buf,
		makefileData(desktop, "com.example.notes", "notes", "com.example.notes.svg")))
	out := buf.String()
	assert.Contains(t, out, `Exec := "notes"`+"\n")
	assert.Contains(t, out, `Icon := "com.example.notes.svg"`+"\n")
	assert.NotContains(t, out, "%U")
}
