package commands

import (
	"bytes"
	"image"
	"image/png"
	"log"
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
	// Separately, not as one string: Windows checks the template out with CRLF.
	assert.Contains(t, plist.String(), "<key>NSHumanReadableCopyright</key>")
	assert.Contains(t, plist.String(), "<string>"+templates.GopherCredit+"</string>")

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
	// Uninstall removes only what install put there: without an app ID the
	// directory is named after the app, and may be another package's docs.
	// Line by line, not as one string: Windows checks the template out with CRLF.
	assert.Contains(t, mk.String(), "\t-rm $(DESTDIR)$(PREFIX)/share/doc/$(AppID)/CREDITS")
	assert.Contains(t, mk.String(), "\t-rmdir $(DESTDIR)$(PREFIX)/share/doc/$(AppID)")
	assert.Contains(t, mk.String(), "\t-rm $(DESTDIR)$(HOME)/.local/share/doc/$(AppID)/CREDITS")
	assert.Contains(t, mk.String(), "\t-rmdir $(DESTDIR)$(HOME)/.local/share/doc/$(AppID)")
	assert.NotContains(t, mk.String(), "rm -r $(DESTDIR)$(PREFIX)/share/doc")
	assert.NotContains(t, mk.String(), "rm -r $(DESTDIR)$(HOME)/.local/share/doc")
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
	// CC BY 4.0 asks to say when the work was adapted, as GopherCredit does.
	assert.Contains(t, buf.String(), "Gopher artwork based on the Go gopher by Renée French")
	assert.Contains(t, buf.String(), `href="https://creativecommons.org/licenses/by/4.0/"`)
}

// packageUNIX itself ships the credits file and the Makefile line that
// installs it, under the app ID or, without one, the app's name. It leaves no
// staging directory behind, which takes every file it wrote being closed on
// Windows. FreeBSD skips the Linux-only CEF check, so any file will do as the
// executable.
func TestUnixArchiveCarriesTheCredits(t *testing.T) {
	if _, err := exec.LookPath("tar"); err != nil {
		t.Skip("needs tar")
	}
	for _, tc := range []struct{ appID, docDir string }{
		{"com.example.notes", "com.example.notes"},
		{"", "Notes"},
	} {
		t.Run("doc dir "+tc.docDir, func(t *testing.T) {
			dir := t.TempDir()
			exe := filepath.Join(dir, "notes")
			require.NoError(t, os.WriteFile(exe, []byte("not a real binary"), 0o755))
			icon := filepath.Join(dir, "icon.png")
			require.NoError(t, os.WriteFile(icon, templates.DefaultIcon, 0o644))

			p := &Packager{
				appData: &appData{Name: "Notes", AppID: tc.appID, icon: icon},
				os:      "freebsd", dir: dir, exe: exe, defaultIcon: true,
			}
			require.NoError(t, p.packageUNIX())
			assert.NoDirExists(t, filepath.Join(dir, "tmp-pkg"))

			var list bytes.Buffer
			cmd := exec.Command("tar", "-tJf", filepath.Join(dir, "Notes.tar.xz"))
			cmd.Stdout = &list
			require.NoError(t, cmd.Run())
			assert.Contains(t, list.String(), "share/doc/"+tc.docDir+"/CREDITS")

			var mk bytes.Buffer
			cmd = exec.Command("tar", "-xOJf", filepath.Join(dir, "Notes.tar.xz"), "notes/Makefile")
			cmd.Stdout = &mk
			require.NoError(t, cmd.Run())
			assert.Contains(t, mk.String(), "AppID := \""+tc.docDir+"\"")
			assert.Contains(t, mk.String(), "share/doc/$(AppID)/CREDITS")
		})
	}
}

// The credit goes into the manifest bellui generates. An Android app with its
// own AndroidManifest.xml is built from that one, so packaging says the credit
// is missing rather than claiming it was added (#362).
func TestOwnAndroidManifestWarnsTheCreditIsMissing(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		defaultIcon, ownManifest bool
		warns                    bool
	}{
		{"default icon, own manifest", true, true, true},
		{"default icon, generated manifest", true, false, false},
		{"own icon, own manifest", false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.ownManifest {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "AndroidManifest.xml"), []byte("<manifest/>"), 0o644))
			}
			var out bytes.Buffer
			log.SetOutput(&out)
			t.Cleanup(func() { log.SetOutput(os.Stderr) })

			p := &Packager{appData: &appData{}, srcDir: dir, defaultIcon: tc.defaultIcon}
			p.warnOwnManifestDropsCredit()
			if tc.warns {
				assert.Contains(t, out.String(), "credit is not added")
				assert.Contains(t, out.String(), templates.GopherCredit)
			} else {
				assert.Empty(t, out.String())
			}
		})
	}
}
