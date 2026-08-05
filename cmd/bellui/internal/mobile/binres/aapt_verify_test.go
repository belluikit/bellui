// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binres

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
)

// buildToolsAapt returns the aapt binary of the newest installed build-tools,
// or skips t when ANDROID_HOME or build-tools are absent. Newest by numeric
// version — the same lexical trap as #296 applies to build-tools directories.
func buildToolsAapt(t *testing.T) string {
	t.Helper()
	sdkdir := os.Getenv("ANDROID_HOME")
	if sdkdir == "" {
		t.Skip("skipping aapt verification: ANDROID_HOME env var not set")
	}
	dir, err := os.ReadDir(filepath.Join(sdkdir, "build-tools"))
	if err != nil {
		t.Skipf("skipping aapt verification: %v", err)
	}

	// parse "35.0.0" or "36.0.0-rc1" into comparable numbers; unparsable
	// segments count as 0.
	version := func(name string) [3]int {
		var v [3]int
		for i, seg := range strings.SplitN(name, ".", 3) {
			seg, _, _ = strings.Cut(seg, "-")
			v[i], _ = strconv.Atoi(seg)
		}
		return v
	}
	less := func(a, b [3]int) bool {
		for i := range a {
			if a[i] != b[i] {
				return a[i] < b[i]
			}
		}
		return false
	}
	newest, newestVersion := "", [3]int{}
	for _, d := range dir {
		if _, err := os.Stat(filepath.Join(sdkdir, "build-tools", d.Name(), "aapt")); err != nil {
			continue
		}
		if v := version(d.Name()); newest == "" || less(newestVersion, v) {
			newest, newestVersion = d.Name(), v
		}
	}
	if newest == "" {
		t.Skip(`skipping aapt verification: no build-tools with aapt installed ("sdkmanager 'build-tools;35.0.0'")`)
	}
	return filepath.Join(sdkdir, "build-tools", newest, "aapt")
}

// TestAaptDumpTemplateManifest is #287's throwaway verification promoted into
// the suite (#297): compile the CLI's template AndroidManifest.xml exactly as
// `bellui package -os android` does — synthesized uses-sdk and icon included —
// then have aapt, the reference implementation, dump the binary and check the
// tree it reads back. This is the one place a self-consistent encode/decode
// bug cannot hide, because the decoder is not ours. Opt-in by environment:
// needs ANDROID_HOME with any build-tools installed.
func TestAaptDumpTemplateManifest(t *testing.T) {
	r := require.New(t)
	aapt := buildToolsAapt(t)

	// binres cannot reach mobile.manifestTmplData (unexported, and that package
	// imports this one), so the template data is a map — which makes a field
	// added to the template render as "<no value>" here rather than fail. The
	// clone turns that silent drift into an error; it is a clone so the option
	// does not leak into the shared template other tests execute.
	tmpl, err := templates.ManifestAndroid.Clone()
	r.NoError(err)
	tmpl = tmpl.Option("missingkey=error")

	var manifest bytes.Buffer
	r.NoError(tmpl.Execute(&manifest, map[string]any{
		"JavaPkgPath":  "com.example.probe",
		"Build":        7,
		"Version":      "1.2.3",
		"Name":         "Probe",
		"Debug":        true,
		"LibName":      "probe",
		"AdaptiveIcon": false,
		"URLSchemes":   []string{"bellui"},
		"MimeTypes":    []string{"text/plain"},
	}))

	// the package-wide test init suppresses the uses-sdk and icon synthesis;
	// this test wants the manifest a real build produces.
	restore := skipSynthesize
	skipSynthesize = false
	t.Cleanup(func() { skipSynthesize = restore })

	bx, err := UnmarshalXML(bytes.NewReader(manifest.Bytes()), true, 30)
	r.NoError(err)
	bin, err := bx.MarshalBinary()
	r.NoError(err)

	// aapt dump reads the manifest out of a zip
	apk := filepath.Join(t.TempDir(), "probe.apk")
	f, err := os.Create(apk)
	r.NoError(err)
	zw := zip.NewWriter(f)
	w, err := zw.Create("AndroidManifest.xml")
	r.NoError(err)
	_, err = w.Write(bin)
	r.NoError(err)
	r.NoError(zw.Close())
	r.NoError(f.Close())

	out, err := exec.Command(aapt, "dump", "xmltree", apk, "AndroidManifest.xml").CombinedOutput()
	r.NoError(err, "aapt dump xmltree failed:\n%s", out)
	dump := string(out)

	tbl, err := OpenTable()
	r.NoError(err)
	theme, err := tbl.RefByName("@android:style/Theme")
	r.NoError(err)

	// The fourth build-platform attribute, platformBuildVersionName, is the one
	// whose drift #297 found (the old golden claimed "5.0.1-1624448"). aapt
	// keeps the codename as the raw string whether or not it also parses as a
	// decimal — "15" does, a preview codename would not — so the raw is what
	// holds across a SDKPlatform bump; the typed value beside it would not.
	r.Regexp(`(?m)^\s*A: platformBuildVersionName=.*\(Raw: "`+regexp.QuoteMeta(SDKPlatformCodename)+`"\)`, dump)

	for _, want := range []string{
		// three of the four synthesized build-platform attributes (#297); the
		// fourth is asserted above
		fmt.Sprintf("A: android:compileSdkVersion(0x01010572)=(type 0x10)%#x", SDKPlatformVersion),
		fmt.Sprintf("A: android:compileSdkVersionCodename(0x01010573)=%q", SDKPlatformCodename),
		fmt.Sprintf("A: platformBuildVersionCode=(type 0x10)%#x", SDKPlatformVersion),
		// the synthesized uses-sdk element
		fmt.Sprintf("A: android:minSdkVersion(0x0101020c)=(type 0x10)%#x", MinSDK),
		"A: android:targetSdkVersion(0x01010270)=(type 0x10)0x1e",
		// template values across the data types: int, string, bool, flags,
		// reference. 0x22a4 is #287's headline configChanges value —
		// orientation|keyboardHidden|uiMode|locale|layoutDirection.
		"A: android:versionCode(0x0101021b)=(type 0x10)0x7",
		`A: android:versionName(0x0101021c)="1.2.3"`,
		"A: android:configChanges(0x0101001f)=(type 0x11)0x22a4",
		"A: android:exported(0x01010010)=(type 0x12)0xffffffff",
		"A: android:debuggable(0x0101000f)=(type 0x12)0xffffffff",
		"A: android:grantUriPermissions(0x0101001b)=(type 0x12)0xffffffff",
		fmt.Sprintf("A: android:theme(0x01010000)=@0x%08x", uint32(theme)),
		// the synthesized icon reference into the mipmap table
		"A: android:icon(0x01010002)=@0x7f020000",
		// element structure and plain strings
		"E: uses-sdk",
		`A: android:name(0x01010003)="org.golang.app.GoNativeActivity"`,
		`A: android:name(0x01010003)="android.app.lib_name"`,
		`A: android:value(0x01010024)="probe"`,
		`A: android:scheme(0x01010027)="bellui"`,
		`A: android:mimeType(0x01010026)="text/plain"`,
		`A: android:name(0x01010003)="android.permission.POST_NOTIFICATIONS"`,
	} {
		r.Contains(dump, want)
	}
}
