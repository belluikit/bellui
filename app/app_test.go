package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/test"
)

func TestDummyApp(t *testing.T) {
	app := NewWithID("io.bellui.test")

	app.Quit()
}

func TestCurrentApp(t *testing.T) {
	app := NewWithID("io.bellui.test")

	assert.Equal(t, app, bellui.CurrentApp())
}

func TestBelluiApp_UniqueID(t *testing.T) {
	appID := "io.bellui.test"
	app := NewWithID(appID)

	assert.Equal(t, appID, app.UniqueID())

	meta.ID = "fakedInject"
	app = New()

	assert.Equal(t, meta.ID, app.UniqueID())
}

func TestBelluiApp_SetIcon(t *testing.T) {
	app := NewWithID("io.bellui.test")

	metaIcon := &bellui.StaticResource{StaticName: "Metadata", StaticContent: []byte("?PNG...")}
	SetMetadata(bellui.AppMetadata{
		Icon: metaIcon,
	})

	assert.Equal(t, metaIcon, app.Icon())

	setIcon := &bellui.StaticResource{StaticName: "Test"}
	app.SetIcon(setIcon)

	assert.Equal(t, setIcon, app.Icon())
}

func TestFynaApp_Clipboard(t *testing.T) {
	app := test.NewTempApp(t)
	test.NewTempWindow(t, nil)

	text := "My content from test window"
	cb := app.Clipboard()

	cliboardContent := cb.Content()
	if cliboardContent != "" {
		// Current environment has some content stored in clipboard,
		// set temporary to an empty string to allow test and restore later.
		cb.SetContent("")
	}

	assert.Empty(t, cb.Content())

	cb.SetContent(text)
	assert.Equal(t, text, cb.Content())

	// Restore clipboardContent, if any
	cb.SetContent(cliboardContent)

	// The test app's clipboard supports the optional flavor interfaces and the
	// package-level sugar resolves them through bellui.CurrentApp().
	assert.True(t, bellui.SetClipboardHTML("<b>rich</b>", "rich"))
	html, ok := bellui.ClipboardHTML()
	assert.True(t, ok)
	assert.Equal(t, "<b>rich</b>", html)
	assert.Equal(t, "rich", cb.Content())

	cb.SetContent(cliboardContent)
}
