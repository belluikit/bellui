package commands

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/belluikit/bellui/cmd/bellui/internal/metadata"
	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"
)

func TestFormatDesktopFileList(t *testing.T) {
	assert.Equal(t, "", formatDesktopFileList([]string{}))
	assert.Equal(t, "One;", formatDesktopFileList([]string{"One"}))
	assert.Equal(t, "One;Two;", formatDesktopFileList([]string{"One", "Two"}))
}

func TestDesktopFileSource(t *testing.T) {
	tplData := unixData{
		Name: "Testing",
	}
	buf := &bytes.Buffer{}

	err := templates.DesktopFileUNIX.Execute(buf, tplData)
	assert.Nil(t, err)
	assert.False(t, strings.Contains(buf.String(), "[X-Bellui"))

	tplData.SourceRepo = "https://example.com"
	tplData.SourceDir = "cmd/name"

	err = templates.DesktopFileUNIX.Execute(buf, tplData)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(buf.String(), "[X-Bellui"))
	assert.True(t, strings.Contains(buf.String(), "Repo=https://example.com"))
	assert.True(t, strings.Contains(buf.String(), "Dir=cmd/name"))
}

func TestDesktopFileOpenWith(t *testing.T) {
	tplData := unixData{
		Name:      "Testing",
		Exec:      "tester",
		MimeTypes: "text/plain",
	}
	buf := &bytes.Buffer{}

	err := templates.DesktopFileUNIX.Execute(buf, tplData)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(buf.String(), "MimeType=text/plain"))
}

func TestSplitDesktopFileList(t *testing.T) {
	assert.Nil(t, splitDesktopFileList(""))
	assert.Equal(t, []string{"text/plain"}, splitDesktopFileList("text/plain;"))
	assert.Equal(t, []string{"a", "b"}, splitDesktopFileList("a; b ;;"))
}

func TestDedupeStrings(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, dedupeStrings([]string{"a", "b", "a", "c", "b"}))
	assert.Empty(t, dedupeStrings(nil))
}

func TestAppDataFileMimeTypes(t *testing.T) {
	a := &appData{FileAssociations: []metadata.FileAssociation{
		{MimeType: "text/plain"},
		{MimeType: "text/plain"},       // duplicate is collapsed
		{Extensions: []string{"bin"}},  // no mime type is skipped
		{MimeType: "application/json"}, // distinct type is kept
	}}
	assert.Equal(t, []string{"text/plain", "application/json"}, a.fileMimeTypes())
}

// TestDesktopFileSchemeHandlers verifies that the combined MimeType= list rendered
// into a .desktop file includes both document mime types and x-scheme-handler entries.
func TestDesktopFileSchemeHandlers(t *testing.T) {
	mimeList := dedupeStrings(append([]string{"text/plain"}, "x-scheme-handler/testing"))
	tplData := unixData{
		Name:      "Testing",
		Exec:      "tester %U",
		MimeTypes: formatDesktopFileList(mimeList),
	}
	buf := &bytes.Buffer{}

	err := templates.DesktopFileUNIX.Execute(buf, tplData)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(buf.String(), "MimeType=text/plain;x-scheme-handler/testing;"))
	assert.True(t, strings.Contains(buf.String(), "Exec=tester %U"))
}

func TestDesktopFile(t *testing.T) {
	tplData := unixData{
		Name:           "Testing",
		StartupWMClass: "Testing",
	}
	buf := &bytes.Buffer{}

	err := templates.DesktopFileUNIX.Execute(buf, tplData)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(buf.String(), "StartupWMClass=Testing"))
}

const testAppTarFiles = `testapp/
testapp/Makefile
testapp/usr/
testapp/usr/local/
testapp/usr/local/bin/
testapp/usr/local/bin/testapp
testapp/usr/local/share/
testapp/usr/local/share/applications/
testapp/usr/local/share/applications/testapp.desktop
testapp/usr/local/share/pixmaps/
testapp/usr/local/share/pixmaps/testapp.png
`

func TestPackageLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping test that needs linux as host os")
		return
	}

	if os.Getenv("BELLUI_TEST_PACKAGE_LINUX") != "yes" {
		t.Skip("skipping test that needs all Bellui libraries")
		return
	}

	basedir := t.TempDir()
	dir := filepath.Join(basedir, "testapp")

	assert.NoError(t, os.Mkdir(dir, 0o755), "create dir for test app")
	assert.NoError(t, os.Chdir(dir), "change into new app dir")
	var iconBuf bytes.Buffer
	assert.NoError(t, png.Encode(&iconBuf, image.NewRGBA(image.Rect(0, 0, 1, 1))))
	assert.NoError(t, os.WriteFile("Icon.png", iconBuf.Bytes(), 0o644))

	app := &cli.App{
		Name: "bellui",
		Commands: []*cli.Command{
			Init(),
			Translate(), // init uses translate
			Package(),
		},
	}

	assert.NoError(t, app.Run([]string{"bellui", "init", "-v"}), "bellui init")
	assert.NoError(t, app.Run([]string{"bellui", "package", "-os", "linux"}), "bellui package")

	buf := &bytes.Buffer{}
	cmd := exec.Command("tar", "-tJf", "testapp.tar.xz")
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	assert.NoError(t, cmd.Run(), "show archive")
	assert.Equal(t, testAppTarFiles, buf.String())
}
