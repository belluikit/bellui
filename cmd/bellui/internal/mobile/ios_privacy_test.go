package mobile

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// privacyReasons flattens a merged manifest's NSPrivacyAccessedAPITypes into
// category → reasons, in order.
func privacyReasons(t *testing.T, m *plistValue) map[string][]string {
	t.Helper()
	types := m.get(privacyAPITypesKey)
	require.NotNil(t, types)
	out := map[string][]string{}
	for _, entry := range types.Items {
		cat := entry.get(privacyAPITypeKey).Text
		_, dup := out[cat]
		require.False(t, dup, "category %s listed twice", cat)
		out[cat] = []string{}
		for _, r := range entry.get(privacyReasonsKey).Items {
			out[cat] = append(out[cat], r.Text)
		}
	}
	return out
}

func TestIOSPrivacyInfoDefault(t *testing.T) {
	data, err := iosPrivacyInfo(t.TempDir())
	require.NoError(t, err)
	plutilLint(t, data)

	m, err := parsePlist(data)
	require.NoError(t, err)
	assert.Equal(t, map[string][]string{
		"NSPrivacyAccessedAPICategoryFileTimestamp":  {"C617.1"},
		"NSPrivacyAccessedAPICategorySystemBootTime": {"35F9.1"},
	}, privacyReasons(t, m))
	// Tracking and collected data are the app's to declare, never defaulted.
	assert.Equal(t, []string{privacyAPITypesKey}, m.Keys)
}

func TestIOSPrivacyInfoMergesAppFragment(t *testing.T) {
	dir := t.TempDir()
	writeIOSFile(t, dir, iosPrivacyManifest, `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>NSPrivacyTracking</key>
	<false/>
	<key>NSPrivacyAccessedAPITypes</key>
	<array>
		<dict>
			<key>NSPrivacyAccessedAPIType</key>
			<string>NSPrivacyAccessedAPICategoryFileTimestamp</string>
			<key>NSPrivacyAccessedAPITypeReasons</key>
			<array>
				<string>3B52.1</string>
				<string>C617.1</string>
			</array>
		</dict>
		<dict>
			<key>NSPrivacyAccessedAPIType</key>
			<string>NSPrivacyAccessedAPICategoryUserDefaults</string>
			<key>NSPrivacyAccessedAPITypeReasons</key>
			<array>
				<string>CA92.1</string>
			</array>
		</dict>
	</array>
	<key>NSPrivacyCollectedDataTypes</key>
	<array/>
</dict>
</plist>
`)
	data, err := iosPrivacyInfo(dir)
	require.NoError(t, err)
	plutilLint(t, data)

	m, err := parsePlist(data)
	require.NoError(t, err)
	assert.Equal(t, map[string][]string{
		"NSPrivacyAccessedAPICategoryFileTimestamp":  {"C617.1", "3B52.1"},
		"NSPrivacyAccessedAPICategorySystemBootTime": {"35F9.1"},
		"NSPrivacyAccessedAPICategoryUserDefaults":   {"CA92.1"},
	}, privacyReasons(t, m))
	assert.Equal(t, []string{privacyAPITypesKey, "NSPrivacyTracking", "NSPrivacyCollectedDataTypes"}, m.Keys)
	assert.Equal(t, "false", m.get("NSPrivacyTracking").Kind)
}

func TestIOSPrivacyInfoRejectsMalformedFragment(t *testing.T) {
	wrap := func(body string) string {
		return `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict><key>NSPrivacyAccessedAPITypes</key>` + body + `</dict></plist>`
	}
	for name, tc := range map[string]struct{ body, want string }{
		"not an array":     {`<dict/>`, "must be an <array>"},
		"item not a dict":  {`<array><string>x</string></array>`, "item 0 must be a <dict>"},
		"no category":      {`<array><dict><key>NSPrivacyAccessedAPITypeReasons</key><array><string>C617.1</string></array></dict></array>`, "needs a NSPrivacyAccessedAPIType string"},
		"no reasons":       {`<array><dict><key>NSPrivacyAccessedAPIType</key><string>NSPrivacyAccessedAPICategoryDiskSpace</string></dict></array>`, "needs a non-empty NSPrivacyAccessedAPITypeReasons array"},
		"empty reasons":    {`<array><dict><key>NSPrivacyAccessedAPIType</key><string>NSPrivacyAccessedAPICategoryDiskSpace</string><key>NSPrivacyAccessedAPITypeReasons</key><array/></dict></array>`, "needs a non-empty"},
		"reason not a str": {`<array><dict><key>NSPrivacyAccessedAPIType</key><string>NSPrivacyAccessedAPICategoryDiskSpace</string><key>NSPrivacyAccessedAPITypeReasons</key><array><integer>1</integer></array></dict></array>`, "every reason must be a <string>"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeIOSFile(t, dir, iosPrivacyManifest, wrap(tc.body))
			_, err := iosPrivacyInfo(dir)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "ios/PrivacyInfo.xcprivacy: ")
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// The manifest only reaches the app if the Xcode project copies it: a file
// reference in the main group and a build file in the Resources phase.
func TestIOSProjectBundlesPrivacyManifest(t *testing.T) {
	var proj bytes.Buffer
	require.NoError(t, projPbxprojTmpl.Execute(&proj, iosProjectData("com.example.app", "", "", "TEAM", true)))
	s := proj.String()
	assert.Contains(t, s, `B5E1F0D00000000000000000 /* PrivacyInfo.xcprivacy */ = {isa = PBXFileReference; lastKnownFileType = text.xml; path = PrivacyInfo.xcprivacy; sourceTree = "<group>"; };`)
	assert.Contains(t, s, "B5E1F0D10000000000000000 /* PrivacyInfo.xcprivacy in Resources */ = {isa = PBXBuildFile; fileRef = B5E1F0D00000000000000000")
	assert.Contains(t, s, "254BB84E1B1FD08900C56DE9 /* Images.xcassets */,\n        B5E1F0D00000000000000000 /* PrivacyInfo.xcprivacy */,")
	assert.Contains(t, s, "254BB84F1B1FD08900C56DE9 /* Images.xcassets in Resources */,\n        B5E1F0D10000000000000000 /* PrivacyInfo.xcprivacy in Resources */,")
}

// requiredReasonSymbols maps the C functions Apple lists as required-reason
// APIs to their categories (BundleResources, NSPrivacyAccessedAPIType, read
// 2026-11-03). The getattrlist family is listed under both file timestamps
// and disk space. Objective-C and Swift APIs are left out: a Go program
// cannot call them without cgo code of its own.
var requiredReasonSymbols = map[string][]string{
	"_stat":               {"NSPrivacyAccessedAPICategoryFileTimestamp"},
	"_fstat":              {"NSPrivacyAccessedAPICategoryFileTimestamp"},
	"_fstatat":            {"NSPrivacyAccessedAPICategoryFileTimestamp"},
	"_lstat":              {"NSPrivacyAccessedAPICategoryFileTimestamp"},
	"_getattrlist":        {"NSPrivacyAccessedAPICategoryFileTimestamp", "NSPrivacyAccessedAPICategoryDiskSpace"},
	"_getattrlistbulk":    {"NSPrivacyAccessedAPICategoryFileTimestamp"},
	"_fgetattrlist":       {"NSPrivacyAccessedAPICategoryFileTimestamp", "NSPrivacyAccessedAPICategoryDiskSpace"},
	"_getattrlistat":      {"NSPrivacyAccessedAPICategoryFileTimestamp", "NSPrivacyAccessedAPICategoryDiskSpace"},
	"_mach_absolute_time": {"NSPrivacyAccessedAPICategorySystemBootTime"},
	"_statfs":             {"NSPrivacyAccessedAPICategoryDiskSpace"},
	"_statvfs":            {"NSPrivacyAccessedAPICategoryDiskSpace"},
	"_fstatfs":            {"NSPrivacyAccessedAPICategoryDiskSpace"},
	"_fstatvfs":           {"NSPrivacyAccessedAPICategoryDiskSpace"},
}

// TestIOSRuntimeAPIReasonsCoverGo pins iosRuntimeAPIReasons to what the Go
// runtime really imports: it builds a plain Go program for ios/arm64 with the
// installed toolchain and fails when the binary imports a required-reason API
// whose category the toolkit does not declare — a Go upgrade that starts
// calling statfs would otherwise get every bellui app refused at upload.
// It needs Xcode's iOS SDK; BELLUI_REQUIRE_IOS_SDK=1 makes its absence a
// failure (the macOS CI leg sets it).
func TestIOSRuntimeAPIReasonsCoverGo(t *testing.T) {
	sdk, err := exec.Command("xcrun", "--sdk", "iphoneos", "--show-sdk-path").Output()
	if runtime.GOOS != "darwin" || err != nil {
		if os.Getenv("BELLUI_REQUIRE_IOS_SDK") == "1" {
			t.Fatalf("no iOS SDK (GOOS %s, xcrun: %v)", runtime.GOOS, err)
		}
		t.Skip("needs macOS with Xcode's iOS SDK")
	}
	clang, err := exec.Command("xcrun", "--sdk", "iphoneos", "-f", "clang").Output()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module hello\n\ngo 1.24\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	start := time.Now()
	fi, err := os.Stat(os.Args[0])
	fmt.Println(fi, err, time.Since(start))
}
`), 0o644))
	bin := filepath.Join(dir, "hello")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOOS=ios", "GOARCH=arm64", "CGO_ENABLED=1", "GOFLAGS=",
		"CC="+strings.TrimSpace(string(clang))+" -isysroot "+strings.TrimSpace(string(sdk))+" -arch arm64 -miphoneos-version-min=15.0")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", out)

	nm, err := exec.Command("xcrun", "nm", "-u", bin).Output()
	require.NoError(t, err)
	declared := map[string]bool{}
	for _, r := range iosRuntimeAPIReasons {
		declared[r.Category] = true
	}
	found := 0
	for _, sym := range strings.Fields(string(nm)) {
		for _, cat := range requiredReasonSymbols[sym] {
			found++
			t.Logf("%s → %s", sym, cat)
			assert.True(t, declared[cat], "the Go runtime imports %s (%s), which iosRuntimeAPIReasons does not declare", sym, cat)
		}
	}
	// A Go binary always imports some (stat, mach_absolute_time); none means
	// nm read the wrong thing, not that the runtime stopped calling them.
	require.NotZero(t, found, "no required-reason import found; nm output:\n%s", nm)
}
