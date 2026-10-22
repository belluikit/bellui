package mobile

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/belluikit/bellui/cmd/bellui/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAppManifest = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.app"
	android:versionCode="1" android:versionName="1.0">
	<application android:label="App" android:debuggable="true">
	<activity android:name="org.golang.app.GoNativeActivity" android:exported="true">
		<meta-data android:name="android.app.lib_name" android:value="app" />
		<intent-filter>
			<action android:name="android.intent.action.MAIN" />
			<category android:name="android.intent.category.LAUNCHER" />
		</intent-filter>
	</activity>
	<meta-data android:name="shared.key" android:value="from-app" />
	</application>
	<uses-permission android:name="android.permission.INTERNET" />
</manifest>`

const testAARManifest = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:tools="http://schemas.android.com/tools" package="com.example.aar">
    <uses-sdk android:minSdkVersion="14" />
    <uses-permission android:name="android.permission.INTERNET" />
    <uses-permission android:name="android.permission.VIBRATE" tools:node="merge" />
    <queries><package android:name="com.example.other" /></queries>
    <application android:appComponentFactory="x.Factory">
        <meta-data android:name="shared.key" android:value="from-lib" />
        <meta-data android:name="com.example.aar.greeting" android:value="@string/aar_greeting" />
        <provider android:name="com.example.aar.Provider"
            android:authorities="${applicationId}.aarprovider" android:exported="false" />
    </application>
</manifest>`

func writeZip(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	f, err := os.Create(path)
	require.NoError(t, err)
	zw := zip.NewWriter(f)
	for name, data := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())
}

func TestMergeLibraryManifests(t *testing.T) {
	work := t.TempDir()
	dir := t.TempDir()
	writeZip(t, filepath.Join(dir, "android", "libs", "lib.aar"), map[string][]byte{
		"AndroidManifest.xml": []byte(testAARManifest),
	})
	libs, err := collectAndroidLibs(dir, work)
	require.NoError(t, err)
	require.Len(t, libs.aars, 1)
	assert.Equal(t, "com.example.aar", libs.aars[0].pkg)

	merged, err := mergeLibraryManifests([]byte(testAppManifest), libs)
	require.NoError(t, err)
	m := string(merged)

	// New entries arrive, with ${applicationId} resolved to the app package.
	assert.Contains(t, m, `<uses-permission android:name="android.permission.VIBRATE"></uses-permission>`)
	assert.Contains(t, m, `<queries><package android:name="com.example.other"></package></queries>`)
	assert.Contains(t, m, `android:authorities="com.example.app.aarprovider"`)
	assert.Contains(t, m, `<meta-data android:name="com.example.aar.greeting" android:value="@string/aar_greeting">`)
	// The app's own entries win: no duplicate permission, no second shared.key.
	assert.Equal(t, 1, strings.Count(m, `"android.permission.INTERNET"`))
	assert.Equal(t, 1, strings.Count(m, `"shared.key"`))
	assert.Contains(t, m, `android:value="from-app"`)
	// tools:* is dropped, and uses-sdk and <application> attributes are not merged.
	assert.NotContains(t, m, "tools:")
	assert.NotContains(t, m, "appComponentFactory")
	assert.Equal(t, 0, strings.Count(m, "uses-sdk"))
	// Application children land inside <application>, top-level ones outside.
	appEnd := strings.Index(m, "</application>")
	assert.Less(t, strings.Index(m, "com.example.aar.Provider"), appEnd)
	assert.Greater(t, strings.Index(m, "android.permission.VIBRATE"), appEnd)
}

func TestMergeLibraryManifestsWithoutAARsIsIdentity(t *testing.T) {
	merged, err := mergeLibraryManifests([]byte(testAppManifest), &androidLibs{jars: []string{"x.jar"}})
	require.NoError(t, err)
	assert.Equal(t, testAppManifest, string(merged))
}

func TestCollectAndroidLibsAbsent(t *testing.T) {
	libs, err := collectAndroidLibs(t.TempDir(), t.TempDir())
	require.NoError(t, err)
	assert.True(t, libs.empty())
}

func TestAARNativeLibsNeedEveryBuiltABI(t *testing.T) {
	dir := t.TempDir()
	writeZip(t, filepath.Join(dir, "android", "libs", "native.aar"), map[string][]byte{
		"AndroidManifest.xml":          []byte(`<manifest package="com.example.native"/>`),
		"jni/arm64-v8a/libnative.so":   []byte("so"),
		"jni/armeabi-v7a/libnative.so": []byte("so"),
	})
	libs, err := collectAndroidLibs(dir, t.TempDir())
	require.NoError(t, err)

	got, err := libs.nativeLibs([]string{"arm64-v8a"}, "libapp.so")
	require.NoError(t, err)
	assert.Equal(t, []string{"lib/arm64-v8a/libnative.so"}, mapKeys(got))

	_, err = libs.nativeLibs([]string{"arm64-v8a", "x86_64"}, "libapp.so")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not for x86_64")

	_, err = libs.nativeLibs([]string{"arm64-v8a"}, "libnative.so")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "app's own library name")
}

func TestExtractAARRejectsEscapingEntries(t *testing.T) {
	dir := t.TempDir()
	writeZip(t, filepath.Join(dir, "android", "libs", "evil.aar"), map[string][]byte{
		"AndroidManifest.xml": []byte(`<manifest package="x"/>`),
		"../../escape.txt":    []byte("x"),
	})
	_, err := collectAndroidLibs(dir, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes")
}

func mapKeys(m map[string]string) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// sdkTools returns the Android SDK pieces the library pipeline runs, or skips.
func sdkTools(t *testing.T) (androidJar, buildTools string) {
	t.Helper()
	home := os.Getenv("ANDROID_HOME")
	if home == "" {
		t.Skip("ANDROID_HOME not set: the android/libs packaging test needs javac, d8 and aapt2")
	}
	if _, err := exec.LookPath("javac"); err != nil {
		t.Skip("javac not on PATH")
	}
	platform, err := findLastPlatform(home)
	if err != nil {
		t.Skip(err)
	}
	buildTools = util.AndroidBuildToolsPath()
	for _, tool := range []string{"d8", "aapt2", "dexdump"} {
		if !util.Exists(filepath.Join(buildTools, tool)) {
			t.Skipf("%s not in %s", tool, buildTools)
		}
	}
	return filepath.Join(platform, "android.jar"), buildTools
}

// javacJar compiles sources (path → source) and returns the classes as jar
// entries, leaving out classes whose name starts with skipPrefix.
func javacJar(t *testing.T, androidJar string, classpath []string, sources map[string]string, skipPrefix string) map[string][]byte {
	t.Helper()
	src, out := t.TempDir(), t.TempDir()
	var files []string
	for name, body := range sources {
		p := filepath.Join(src, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
		files = append(files, p)
	}
	args := []string{"-source", "1.8", "-target", "1.8", "-bootclasspath", androidJar, "-d", out}
	if len(classpath) > 0 {
		args = append(args, "-classpath", strings.Join(classpath, string(os.PathListSeparator)))
	}
	cmd := exec.Command("javac", append(args, files...)...)
	b, err := cmd.CombinedOutput()
	require.NoError(t, err, string(b))
	entries := map[string][]byte{}
	require.NoError(t, filepath.Walk(out, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(out, p)
		rel = filepath.ToSlash(rel)
		if skipPrefix != "" && strings.HasPrefix(rel, skipPrefix) {
			return nil
		}
		data, err := os.ReadFile(p)
		entries[rel] = data
		return err
	}))
	return entries
}

// TestPackageAndroidLibs runs the android/libs pipeline with the real SDK
// tools — everything goAndroidBuild does except compiling the Go library —
// and inspects the APK: dex classes, the binary manifest, the resource table,
// the native library and the assets (#672).
func TestPackageAndroidLibs(t *testing.T) {
	androidJar, buildTools := sdkTools(t)
	prevTmp := tmpdir
	tmpdir = t.TempDir()
	t.Cleanup(func() { tmpdir = prevTmp })

	app := t.TempDir()
	libDir := filepath.Join(app, "android", "libs")

	// A plain jar.
	plain := javacJar(t, androidJar, nil, map[string]string{
		"com/example/plain/Plain.java": `package com.example.plain; public class Plain { public static String hi() { return "plain"; } }`,
	}, "")
	writeZip(t, filepath.Join(libDir, "plain.jar"), plain)

	// An AAR whose class reads its own R, compiled the way AGP does: against a
	// non-final stub R that the AAR then leaves out.
	aarClasses := javacJar(t, androidJar, nil, map[string]string{
		"com/example/aar/R.java":       `package com.example.aar; public final class R { public static final class string { public static int aar_greeting = 0; } }`,
		"com/example/aar/Greeter.java": `package com.example.aar; public class Greeter { public static int id() { return R.string.aar_greeting; } }`,
	}, "com/example/aar/R")
	var classesJar bytes.Buffer
	zw := zip.NewWriter(&classesJar)
	for name, data := range aarClasses {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, _ = w.Write(data)
	}
	require.NoError(t, zw.Close())
	writeZip(t, filepath.Join(libDir, "greeter.aar"), map[string][]byte{
		"AndroidManifest.xml":         []byte(testAARManifest),
		"classes.jar":                 classesJar.Bytes(),
		"res/values/values.xml":       []byte(`<resources><string name="aar_greeting">hello from aar</string></resources>`),
		"jni/arm64-v8a/libgreet.so":   []byte("not really a library"),
		"assets/greeter/data.txt":     []byte("lib asset"),
		"assets/shared.txt":           []byte("lib version"),
		"aidl/com/example/Thing.aidl": []byte("ignored"),
	})

	// An AAR with no res/ whose code reads a dependency's resource through
	// its own R (transitive R, as R.txt lists).
	userClasses := javacJar(t, androidJar, nil, map[string]string{
		"com/example/user/R.java":    `package com.example.user; public final class R { public static final class string { public static int aar_greeting = 0; } }`,
		"com/example/user/User.java": `package com.example.user; public class User { public static int id() { return R.string.aar_greeting; } }`,
	}, "com/example/user/R")
	var userJar bytes.Buffer
	zw = zip.NewWriter(&userJar)
	for name, data := range userClasses {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, _ = w.Write(data)
	}
	require.NoError(t, zw.Close())
	writeZip(t, filepath.Join(libDir, "user.aar"), map[string][]byte{
		"AndroidManifest.xml": []byte(`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.user"/>`),
		"classes.jar":         userJar.Bytes(),
		"R.txt":               []byte("int string aar_greeting 0x7f010000\n"),
	})

	// App Java that uses both libraries.
	require.NoError(t, os.WriteFile(filepath.Join(app, "android", "Shim.java"), []byte(
		`package com.example.app; public class Shim { public static String go() { return com.example.plain.Plain.hi() + com.example.aar.Greeter.id(); } }`,
	), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(app, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(app, "assets", "shared.txt"), []byte("app version"), 0o644))

	libs, err := prepareAndroidLibs(app, []byte(testAppManifest), []string{"arm64"}, "app", "", "", "", "", 35, 7, "1.2")
	require.NoError(t, err)
	assert.Equal(t, []string{"lib/arm64-v8a/libgreet.so"}, libs.nativeFiles)
	require.NotNil(t, libs.linked)

	dex, err := mergeAppJavaDex(app, libs.libs.jars, libs.rJava())
	require.NoError(t, err)
	require.NotEmpty(t, dex)

	goLib := "lib/arm64-v8a/libapp.so"
	require.NoError(t, os.MkdirAll(filepath.Join(tmpdir, "lib", "arm64-v8a"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpdir, goLib), []byte("go"), 0o644))

	apkPath := filepath.Join(t.TempDir(), "app.apk")
	f, err := os.Create(apkPath)
	require.NoError(t, err)
	apkw, err := buildAPK(f, nil, append([]string{goLib}, libs.nativeFiles...), []string{"arm64"}, dex)
	require.NoError(t, err)
	require.NoError(t, addAssets(apkw, []byte(testAppManifest), app, "", 35, 7, "1.2", "", "", "", libs))
	require.NoError(t, apkw.Close())
	require.NoError(t, f.Close())

	zr, err := zip.OpenReader(apkPath)
	require.NoError(t, err)
	defer zr.Close()
	entries := map[string]*zip.File{}
	for _, e := range zr.File {
		entries[e.Name] = e
	}
	for _, name := range []string{
		"classes.dex", "resources.arsc", "AndroidManifest.xml", goLib,
		"lib/arm64-v8a/libgreet.so", "assets/greeter/data.txt", "assets/shared.txt",
	} {
		assert.Contains(t, entries, name)
	}
	assert.Equal(t, "app version", readZipEntry(t, entries["assets/shared.txt"]), "the app's asset wins")

	// Every class is in the dex: bellui's own, the app's, both libraries' and
	// the generated library R.
	var descriptors strings.Builder
	for _, e := range zr.File {
		if !strings.HasPrefix(e.Name, "classes") || !strings.HasSuffix(e.Name, ".dex") {
			continue
		}
		p := filepath.Join(t.TempDir(), e.Name)
		require.NoError(t, os.WriteFile(p, []byte(readZipEntry(t, e)), 0o644))
		out, err := exec.Command(filepath.Join(buildTools, "dexdump"), p).CombinedOutput()
		require.NoError(t, err, string(out))
		descriptors.Write(out)
	}
	for _, class := range []string{
		"Lorg/golang/app/GoNativeActivity;",
		"Lcom/example/app/Shim;",
		"Lcom/example/plain/Plain;",
		"Lcom/example/aar/Greeter;",
		"Lcom/example/aar/R$string;",
		"Lcom/example/user/User;",
		"Lcom/example/user/R$string;",
	} {
		assert.Contains(t, descriptors.String(), "Class descriptor  : '"+class+"'")
	}

	// The binary manifest carries the merged entries; the library's string
	// resource is in the table.
	aapt2 := filepath.Join(buildTools, "aapt2")
	xmltree, err := exec.Command(aapt2, "dump", "xmltree", "--file", "AndroidManifest.xml", apkPath).CombinedOutput()
	require.NoError(t, err, string(xmltree))
	assert.Contains(t, string(xmltree), `"android.permission.VIBRATE"`)
	assert.Contains(t, string(xmltree), `"com.example.aar.Provider"`)
	assert.Contains(t, string(xmltree), `"com.example.app.aarprovider"`)
	assert.Contains(t, string(xmltree), `"com.example.aar.greeting"`)
	res, err := exec.Command(aapt2, "dump", "resources", apkPath).CombinedOutput()
	require.NoError(t, err, string(res))
	assert.Contains(t, string(res), "string/aar_greeting")
	assert.Contains(t, string(res), `"hello from aar"`)
}

// Jars alone change only the dex: the resources stay on the binres path, so
// the manifest is byte-identical to a build without libraries.
func TestPackageAndroidJarsOnlyKeepsBinresPath(t *testing.T) {
	androidJar, buildTools := sdkTools(t)
	prevTmp := tmpdir
	tmpdir = t.TempDir()
	t.Cleanup(func() { tmpdir = prevTmp })

	app := t.TempDir()
	writeZip(t, filepath.Join(app, "android", "libs", "plain.jar"), javacJar(t, androidJar, nil, map[string]string{
		"com/example/plain/Plain.java": `package com.example.plain; public class Plain {}`,
	}, ""))
	libs, err := prepareAndroidLibs(app, []byte(testAppManifest), []string{"arm64"}, "app", "", "", "", "", 35, 1, "1")
	require.NoError(t, err)
	assert.Nil(t, libs.linked)
	assert.Empty(t, libs.nativeFiles)

	dex, err := mergeAppJavaDex(app, libs.libs.jars, libs.rJava())
	require.NoError(t, err)
	require.NotEmpty(t, dex, "a jar alone is dexed even without android/*.java")
	out, err := exec.Command(filepath.Join(buildTools, "dexdump"), dex[0]).CombinedOutput()
	require.NoError(t, err, string(out))
	assert.Contains(t, string(out), "'Lcom/example/plain/Plain;'")

	manifest := func(l *preparedLibs) []byte {
		var apk bytes.Buffer
		apkw, err := buildAPK(&apk, nil, nil, nil, nil)
		require.NoError(t, err)
		require.NoError(t, addAssets(apkw, []byte(testAppManifest), app, "", 35, 1, "1", "", "", "", l))
		require.NoError(t, apkw.Close())
		zr, err := zip.NewReader(bytes.NewReader(apk.Bytes()), int64(apk.Len()))
		require.NoError(t, err)
		for _, e := range zr.File {
			if e.Name == "AndroidManifest.xml" {
				return []byte(readZipEntry(t, e))
			}
		}
		t.Fatal("no manifest")
		return nil
	}
	assert.Equal(t, manifest(&preparedLibs{libs: &androidLibs{}}), manifest(libs))
}

func readZipEntry(t *testing.T, f *zip.File) string {
	t.Helper()
	rc, err := f.Open()
	require.NoError(t, err)
	defer rc.Close()
	b, err := io.ReadAll(rc)
	require.NoError(t, err)
	return string(b)
}

// TestFindLastPlatform pins numeric API-level ordering: name order would
// pick android-9 over android-35, and a codename preview over both.
func TestFindLastPlatform(t *testing.T) {
	for _, tc := range []struct {
		dirs []string
		want string
	}{
		{[]string{"android-35", "android-9"}, "android-35"},
		{[]string{"android-34", "android-34-ext12", "android-33"}, "android-34-ext12"},
		{[]string{"android-36", "android-36.1", "android-35"}, "android-36.1"},
		{[]string{"android-Baklava", "android-35"}, "android-35"},
		{[]string{"android-Baklava"}, "android-Baklava"},
	} {
		home := t.TempDir()
		for _, d := range tc.dirs {
			if err := os.MkdirAll(filepath.Join(home, "platforms", d), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		got, err := findLastPlatform(home)
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(got) != tc.want {
			t.Errorf("findLastPlatform(%v) = %s, want %s", tc.dirs, filepath.Base(got), tc.want)
		}
	}
}
