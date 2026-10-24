package commands

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
	"golang.org/x/mod/modfile"

	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
)

// requireAndroidNDKEnv, set to 1 where CI promises an Android SDK and NDK,
// turns the skip of a test that needs them into a failure, so the leg that
// is meant to run it cannot pass by skipping.
const requireAndroidNDKEnv = "BELLUI_REQUIRE_ANDROID_NDK"

// androidNDKOrSkip skips t unless an Android build can run here: ANDROID_HOME
// and an NDK where bellui looks for one ($ANDROID_NDK_HOME, or
// $ANDROID_HOME/ndk-bundle). Under BELLUI_REQUIRE_ANDROID_NDK=1 it fails
// instead.
func androidNDKOrSkip(t *testing.T) {
	t.Helper()
	missing := func(msg string) {
		t.Helper()
		if os.Getenv(requireAndroidNDKEnv) == "1" {
			t.Fatalf("%s=1 but %s", requireAndroidNDKEnv, msg)
		}
		t.Skipf("%s (set %s=1 to make this a failure)", msg, requireAndroidNDKEnv)
	}
	home := os.Getenv("ANDROID_HOME")
	if home == "" {
		missing("ANDROID_HOME is not set")
	}
	for _, ndk := range []string{os.Getenv("ANDROID_NDK_HOME"), filepath.Join(home, "ndk-bundle")} {
		if ndk == "" {
			continue
		}
		if _, err := os.Stat(ndk); err == nil {
			return
		}
	}
	missing("no Android NDK in $ANDROID_NDK_HOME or $ANDROID_HOME/ndk-bundle")
}

// ownAndroidManifest is an app's own AndroidManifest.xml: it names the
// library and the activity, and carries no icon credit.
const ownAndroidManifest = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.credit"
	android:versionCode="1" android:versionName="1.0.0">
	<application android:label="Credit" android:debuggable="true">
		<activity android:name="org.golang.app.GoNativeActivity" android:label="Credit"
			android:exported="true" android:theme="@android:style/Theme">
			<meta-data android:name="android.app.lib_name" android:value="credit" />
			<intent-filter>
				<action android:name="android.intent.action.MAIN" />
				<category android:name="android.intent.category.LAUNCHER" />
			</intent-filter>
		</activity>
	</application>
</manifest>
`

// `bellui package -os android` of an app with its own AndroidManifest.xml and
// no Icon.png says the default icon's credit is not in the package: the
// credit goes into the manifest bellui generates, and this app's manifest is
// used unchanged (#362, #714). TestOwnAndroidManifestWarnsTheCreditIsMissing
// covers the helper's cases; this covers that packageAndroid calls it, which
// takes a real Android build.
func TestPackageAndroidOwnManifestWarnsTheCreditIsMissing(t *testing.T) {
	androidNDKOrSkip(t)

	// The app builds against the toolkit in this checkout, as the CLI does.
	toolkit, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	require.NoError(t, err)
	goMod, err := os.ReadFile(filepath.Join(toolkit, "go.mod"))
	require.NoError(t, err)
	require.Equal(t, "github.com/belluikit/bellui", modfile.ModulePath(goMod))
	goSum, err := os.ReadFile(filepath.Join(toolkit, "go.sum"))
	require.NoError(t, err)

	dir := t.TempDir()
	write := func(name, body string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
	}
	write("main.go", `package main

import (
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("Credit")
	w.SetContent(widget.NewLabel("Credit"))
	w.Show()
	a.Run()
}
`)
	write("go.mod", "module example.com/credit\n\ngo 1.25.0\n\nrequire github.com/belluikit/bellui v0.0.0\n\n"+
		"replace github.com/belluikit/bellui => \""+filepath.ToSlash(toolkit)+"\"\n")
	write("go.sum", string(goSum))
	write("AndroidManifest.xml", ownAndroidManifest)

	// The app's go.mod lists only the toolkit, so the build fills in the
	// rest from the toolkit's go.sum. Host cgo flags (a Homebrew -L, a macOS
	// linker option) would reach the NDK's clang and break the link.
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("CGO_CFLAGS", "")
	t.Setenv("CGO_LDFLAGS", "")
	t.Chdir(dir)

	var out bytes.Buffer
	log.SetOutput(io.MultiWriter(&out, os.Stderr))
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	app := &cli.App{Name: "bellui", Commands: []*cli.Command{Package()}}
	require.NoError(t, app.Run([]string{
		"bellui", "package", "-os", "android/arm64", "--app-id", "com.example.credit", "--name", "Credit",
	}))
	assert.FileExists(t, filepath.Join(dir, "Credit.apk"))
	assert.NoFileExists(t, filepath.Join(dir, "Icon.png"))
	assert.Contains(t, out.String(), "The app's own AndroidManifest.xml is used, so the default icon's credit is not added to it.")
	assert.Contains(t, out.String(), templates.GopherCredit)
}
