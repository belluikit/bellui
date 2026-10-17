package commands

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
)

// creditApp returns a main package directory, with an Icon.png when withIcon.
func creditApp(t *testing.T, withIcon bool) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	if withIcon {
		var buf bytes.Buffer
		require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "Icon.png"), buf.Bytes(), 0o644))
	}
	return dir
}

// The default icon is CC BY 4.0 gopher art: a package that falls back to it
// must carry the credit, and one with its own icon must not (#362).
func TestDefaultIconCarriesItsCredit(t *testing.T) {
	for _, tc := range []struct {
		name     string
		withIcon bool
		want     string
	}{
		{"no Icon.png", false, templates.GopherCredit},
		{"own Icon.png", true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := creditApp(t, tc.withIcon)
			t.Chdir(dir)
			p := &Packager{appData: &appData{}, srcDir: dir, os: "darwin"}
			require.NoError(t, p.validate())
			t.Cleanup(func() { _ = os.RemoveAll(p.tempDir) })
			assert.Equal(t, tc.want, p.iconCredit())
		})
	}
}

func TestIconCreditReachesEveryDesktopPackage(t *testing.T) {
	p := &Packager{appData: &appData{Name: "Notes", AppVersion: "1.0.0", AppBuild: 1}, defaultIcon: true}

	// macOS: Info.plist, shown in the app's About box.
	var plist bytes.Buffer
	require.NoError(t, templates.InfoPlistDarwin.Execute(&plist, p.darwinInfo("notes")))
	assert.Contains(t, plist.String(), "<key>NSHumanReadableCopyright</key>\n\t<string>"+templates.GopherCredit+"</string>")

	// Windows: the exe's version resource, shown in its Properties.
	assert.Equal(t, templates.GopherCredit, p.windowsVersionInfo("i.ico", "m.manifest").StringFileInfo.Comments)

	// Linux/BSD: share/doc/<app>/CREDITS, in the archive and installed.
	share := t.TempDir()
	require.NoError(t, writeUnixCredits(share, "com.example.notes", p.iconCredit()))
	got, err := os.ReadFile(filepath.Join(share, "doc", "com.example.notes", "CREDITS"))
	require.NoError(t, err)
	assert.Contains(t, string(got), templates.GopherCredit)
	var mk bytes.Buffer
	require.NoError(t, templates.MakefileUNIX.Execute(&mk,
		makefileData(unixData{Name: "Notes", Credits: true}, "com.example.notes", "notes", "com.example.notes.png")))
	assert.Contains(t, mk.String(), "share/doc/$(AppID)/CREDITS $(DESTDIR)$(PREFIX)/share/doc/$(AppID)/CREDITS")
	assert.Contains(t, mk.String(), "share/doc/$(AppID)/CREDITS $(DESTDIR)$(HOME)/.local/share/doc/$(AppID)/CREDITS")
}

func TestOwnIconAddsNoCredit(t *testing.T) {
	p := &Packager{appData: &appData{Name: "Notes"}}

	var plist bytes.Buffer
	require.NoError(t, templates.InfoPlistDarwin.Execute(&plist, p.darwinInfo("notes")))
	assert.NotContains(t, plist.String(), "NSHumanReadableCopyright")
	assert.Empty(t, p.windowsVersionInfo("i.ico", "m.manifest").StringFileInfo.Comments)

	share := t.TempDir()
	require.NoError(t, writeUnixCredits(share, "com.example.notes", p.iconCredit()))
	assert.NoDirExists(t, filepath.Join(share, "doc"))
	var mk bytes.Buffer
	require.NoError(t, templates.MakefileUNIX.Execute(&mk,
		makefileData(unixData{Name: "Notes"}, "com.example.notes", "notes", "com.example.notes.png")))
	assert.NotContains(t, mk.String(), "CREDITS")
}

// The web splash screen always shows the gopher spinner, so it always credits it.
func TestWebSplashCreditsTheSpinner(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, templates.IndexHTML.Execute(&buf, webData{AppName: "Notes", AppVersion: "1.0.0"}))
	assert.Contains(t, buf.String(), "Gopher by Renée French")
	assert.Contains(t, buf.String(), `href="https://creativecommons.org/licenses/by/4.0/"`)
}

// packageUNIX itself ships the credits file and the Makefile line that
// installs it. FreeBSD skips the Linux-only CEF check, so any file will do as
// the executable.
func TestUnixArchiveCarriesTheCredits(t *testing.T) {
	if _, err := exec.LookPath("tar"); err != nil {
		t.Skip("needs tar")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "notes")
	require.NoError(t, os.WriteFile(exe, []byte("not a real binary"), 0o755))
	icon := filepath.Join(dir, "icon.png")
	require.NoError(t, os.WriteFile(icon, templates.DefaultIcon, 0o644))

	p := &Packager{
		appData: &appData{Name: "Notes", AppID: "com.example.notes", icon: icon},
		os:      "freebsd", dir: dir, exe: exe, defaultIcon: true,
	}
	require.NoError(t, p.packageUNIX())

	var list bytes.Buffer
	cmd := exec.Command("tar", "-tJf", filepath.Join(dir, "Notes.tar.xz"))
	cmd.Stdout = &list
	require.NoError(t, cmd.Run())
	assert.Regexp(t, `share/doc/com\.example\.notes/CREDITS`, list.String())

	var mk bytes.Buffer
	cmd = exec.Command("tar", "-xOJf", filepath.Join(dir, "Notes.tar.xz"), "notes/Makefile")
	cmd.Stdout = &mk
	require.NoError(t, cmd.Run())
	assert.Contains(t, mk.String(), "share/doc/$(AppID)/CREDITS")
}
