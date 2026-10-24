package commands

import (
	"archive/zip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Android release tests run fake SDK tools: copies of this test binary,
// named after the tool they stand in for (zipalign, jarsigner — .exe on
// Windows, as the real ones are). TestMain sees fakeToolLog in the
// environment and behaves as the tool instead of running the tests.
const (
	fakeToolLog         = "BELLUI_FAKE_TOOL_LOG"
	fakeZipalignFailure = "BELLUI_FAKE_ZIPALIGN_FAIL"
)

func TestMain(m *testing.M) {
	if dir := os.Getenv(fakeToolLog); dir != "" {
		os.Exit(fakeTool(dir, os.Args[1:]))
	}
	os.Exit(m.Run())
}

// fakeTool records its arguments and the password variables it was given in
// dir, as <tool>.args and <tool>.env, one value per line, then does what the
// tool would: zipalign copies its input to its output, or with
// fakeZipalignFailure set writes a partial output and exits 1.
func fakeTool(dir string, args []string) int {
	name := filepath.Base(os.Args[0])
	name = strings.TrimSuffix(name, filepath.Ext(name))
	env := []string{
		"BELLUI_KEYSTORE_PASS=" + os.Getenv("BELLUI_KEYSTORE_PASS"),
		"BELLUI_KEY_PASS=" + os.Getenv("BELLUI_KEY_PASS"),
	}
	if os.WriteFile(filepath.Join(dir, name+".args"), []byte(strings.Join(args, "\n")), 0o644) != nil ||
		os.WriteFile(filepath.Join(dir, name+".env"), []byte(strings.Join(env, "\n")), 0o644) != nil {
		return 2
	}
	if name != "zipalign" {
		return 0
	}
	if len(args) != 3 {
		os.Stderr.WriteString("usage: zipalign <align> <in> <out>\n")
		return 2
	}
	if os.Getenv(fakeZipalignFailure) != "" {
		_ = os.WriteFile(args[2], []byte("partial"), 0o644)
		os.Stderr.WriteString("zipalign: fake failure\n")
		return 1
	}
	data, err := os.ReadFile(args[1])
	if err != nil || os.WriteFile(args[2], data, 0o644) != nil {
		return 2
	}
	return 0
}

// installFakeTool copies the test binary into dir as the program file.
func installFakeTool(t *testing.T, dir, file string) {
	t.Helper()
	exe, err := os.Executable()
	require.NoError(t, err)
	src, err := os.Open(exe)
	require.NoError(t, err)
	defer src.Close()
	dst, err := os.OpenFile(filepath.Join(dir, file), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	require.NoError(t, err)
	_, err = io.Copy(dst, src)
	require.NoError(t, err)
	require.NoError(t, dst.Close())
}

// fakeAndroidSDK points ANDROID_HOME at an SDK whose build-tools hold a fake
// zipalign, puts a fake jarsigner first on PATH, and returns the directory
// the fakes record their calls in.
func fakeAndroidSDK(t *testing.T) string {
	t.Helper()
	prevUtil := util
	util = defaultUtil{}
	t.Cleanup(func() { util = prevUtil })

	exeExt := ""
	if runtime.GOOS == "windows" {
		exeExt = ".exe"
	}
	home := t.TempDir()
	tools := filepath.Join(home, "build-tools", "35.0.0")
	require.NoError(t, os.MkdirAll(tools, 0o755))
	installFakeTool(t, tools, "zipalign"+exeExt)
	jdk := t.TempDir()
	installFakeTool(t, jdk, "jarsigner"+exeExt)

	log := t.TempDir()
	t.Setenv("ANDROID_HOME", home)
	t.Setenv("PATH", jdk+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(fakeToolLog, log)
	t.Setenv("BELLUI_KEYSTORE_PASS", "")
	t.Setenv("BELLUI_KEY_PASS", "")
	return log
}

func readFakeToolLog(t *testing.T, log, file string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(log, file))
	require.NoError(t, err, "the fake tool did not run")
	return strings.Split(string(data), "\n")
}

// A zipalign that fails must leave the package it was given where it was,
// not replace it with zipalign's partial output (#728).
func TestReleaserZipAlignFailureKeepsInput(t *testing.T) {
	log := fakeAndroidSDK(t)
	t.Setenv(fakeZipalignFailure, "1")
	dir := t.TempDir()
	pkg := filepath.Join(dir, "app.aab")
	require.NoError(t, os.WriteFile(pkg, []byte("original"), 0o644))

	err := (&Releaser{}).zipAlign(pkg)
	assert.ErrorContains(t, err, "zipalign")
	assert.ErrorContains(t, err, "fake failure", "zipalign's output is in the error")
	readFakeToolLog(t, log, "zipalign.args")

	data, rerr := os.ReadFile(pkg)
	require.NoError(t, rerr)
	assert.Equal(t, "original", string(data))
	assert.NoFileExists(t, filepath.Join(dir, "unaligned.apk"))
}

// When the package cannot be moved aside, zipAlign says so rather than
// returning success for a package it never aligned (#728).
func TestReleaserZipAlignRenameFailure(t *testing.T) {
	log := fakeAndroidSDK(t)
	err := (&Releaser{}).zipAlign(filepath.Join(t.TempDir(), "missing.aab"))
	assert.ErrorContains(t, err, "moving the package aside")
	assert.NoFileExists(t, filepath.Join(log, "zipalign.args"), "zipalign must not run")
}

func TestReleaserZipAlign(t *testing.T) {
	log := fakeAndroidSDK(t)
	dir := t.TempDir()
	pkg := filepath.Join(dir, "app.aab")
	require.NoError(t, os.WriteFile(pkg, []byte("original"), 0o644))

	require.NoError(t, (&Releaser{}).zipAlign(pkg))
	assert.Equal(t, []string{"16", filepath.Join(dir, "unaligned.apk"), pkg}, readFakeToolLog(t, log, "zipalign.args"))
	data, err := os.ReadFile(pkg)
	require.NoError(t, err)
	assert.Equal(t, "original", string(data))
	assert.NoFileExists(t, filepath.Join(dir, "unaligned.apk"))
}

// Passwords holding the characters cmd.exe and shells treat specially.
const (
	testStorePass = "st0re%PATH%!pw"
	testKeyPass   = "k3y^\"pw"
)

// No signer gets a password on its command line, where the process list
// shows it; both read them from variables set on the child (#729, #674).
func TestReleaserAndroidSignCommandHidesPasswords(t *testing.T) {
	fakeAndroidSDK(t)
	for _, distribution := range []bool{true, false} {
		r := &Releaser{
			Packager:     Packager{distribution: distribution},
			keyStore:     "release.keystore",
			keyStorePass: testStorePass,
			keyName:      "app",
			keyPass:      testKeyPass,
		}
		cmd, err := r.androidSignCommand("app.aab")
		require.NoError(t, err)
		for _, a := range cmd.Args {
			assert.NotContains(t, a, "st0re", "distribution=%v", distribution)
			assert.NotContains(t, a, "k3y", "distribution=%v", distribution)
		}
		assert.Contains(t, cmd.Env, "BELLUI_KEYSTORE_PASS="+testStorePass)
		assert.Contains(t, cmd.Env, "BELLUI_KEY_PASS="+testKeyPass)
		if distribution {
			assert.Equal(t, []string{
				"jarsigner", "-keystore", "release.keystore",
				"-storepass:env", "BELLUI_KEYSTORE_PASS", "-keypass:env", "BELLUI_KEY_PASS",
				"app.aab", "app",
			}, cmd.Args)
		}
	}
}

// The signing step of `bellui release -os android`, run against a fake
// zipalign and a fake jarsigner: the bundle is aligned in place, then signed
// with the passwords passed through the environment only.
func TestReleaserAfterPackageAndroid(t *testing.T) {
	log := fakeAndroidSDK(t)
	dir := t.TempDir()
	r := &Releaser{
		Packager: Packager{
			appData:      &appData{Name: "My App"},
			os:           "android",
			dir:          dir,
			distribution: true,
		},
		keyStore:     filepath.Join(dir, "release.keystore"),
		keyStorePass: testStorePass,
		keyName:      "app",
		keyPass:      testKeyPass,
	}
	pkg := filepath.Join(dir, "My_App.aab")
	require.NoError(t, os.WriteFile(pkg, []byte("bundle"), 0o644))

	require.NoError(t, r.afterPackage())

	assert.Equal(t, []string{"16", filepath.Join(dir, "unaligned.apk"), pkg}, readFakeToolLog(t, log, "zipalign.args"))
	assert.Equal(t, []string{
		"-keystore", r.keyStore,
		"-storepass:env", "BELLUI_KEYSTORE_PASS", "-keypass:env", "BELLUI_KEY_PASS",
		pkg, "app",
	}, readFakeToolLog(t, log, "jarsigner.args"))
	assert.Equal(t, []string{
		"BELLUI_KEYSTORE_PASS=" + testStorePass, "BELLUI_KEY_PASS=" + testKeyPass,
	}, readFakeToolLog(t, log, "jarsigner.env"))
	assert.Empty(t, os.Getenv("BELLUI_KEYSTORE_PASS"), "the password is set on the child only")
	data, err := os.ReadFile(pkg)
	require.NoError(t, err)
	assert.Equal(t, "bundle", string(data))
	assert.NoFileExists(t, filepath.Join(dir, "unaligned.apk"))
}

// The distribution signing command run with the JDK's own jarsigner, on a
// keystore made for the test: jarsigner reads both passwords from the
// variables, characters cmd.exe and shells would alter included. The Windows
// CLI leg requires this test to PASS, so a JDK missing there fails the leg.
func TestReleaserSignsWithJarsigner(t *testing.T) {
	keytool, err := exec.LookPath("keytool")
	if err != nil {
		t.Skip("no JDK keytool on PATH")
	}
	jarsigner, err := exec.LookPath("jarsigner")
	if err != nil {
		t.Skip("no JDK jarsigner on PATH")
	}
	dir := t.TempDir()
	r := &Releaser{
		Packager:     Packager{distribution: true},
		keyStore:     filepath.Join(dir, "release.keystore"),
		keyStorePass: testStorePass,
		keyName:      "app",
		keyPass:      testKeyPass,
	}
	gen := exec.Command(keytool, "-genkeypair", "-keystore", r.keyStore, "-storetype", "JKS",
		"-storepass:env", "BELLUI_KEYSTORE_PASS", "-keypass:env", "BELLUI_KEY_PASS",
		"-alias", r.keyName, "-keyalg", "RSA", "-keysize", "2048", "-validity", "1", "-dname", "CN=bellui test")
	gen.Env = append(os.Environ(), "BELLUI_KEYSTORE_PASS="+testStorePass, "BELLUI_KEY_PASS="+testKeyPass)
	out, err := gen.CombinedOutput()
	require.NoError(t, err, string(out))

	pkg := filepath.Join(dir, "My_App.aab")
	f, err := os.Create(pkg)
	require.NoError(t, err)
	zw := zip.NewWriter(f)
	w, err := zw.Create("base/manifest/AndroidManifest.xml")
	require.NoError(t, err)
	_, err = w.Write([]byte("manifest"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())

	cmd, err := r.androidSignCommand(pkg)
	require.NoError(t, err)
	out, err = cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	out, err = exec.Command(jarsigner, "-verify", pkg).CombinedOutput()
	require.NoError(t, err, string(out))
	assert.Contains(t, string(out), "jar verified.")
}
