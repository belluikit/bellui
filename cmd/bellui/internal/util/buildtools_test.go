package util

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The build-tools file names per host OS, as the SDK ships them: Windows
// carries .exe for the native tools and .bat for the Java ones (#674).
var sdkLayouts = map[string][]string{
	"windows": {"aapt.exe", "aapt2.exe", "dexdump.exe", "zipalign.exe", "d8.bat", "apksigner.bat"},
	"linux":   {"aapt", "aapt2", "dexdump", "zipalign", "d8", "apksigner"},
	"darwin":  {"aapt", "aapt2", "dexdump", "zipalign", "d8", "apksigner"},
}

func fakeBuildTools(t *testing.T, files []string) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, f), nil, 0o755))
	}
	return dir
}

func TestAndroidBuildToolPerOS(t *testing.T) {
	tools := []string{"aapt", "aapt2", "dexdump", "zipalign", "d8", "apksigner"}
	for goos, files := range sdkLayouts {
		t.Run(goos, func(t *testing.T) {
			dir := fakeBuildTools(t, files)
			for i, tool := range tools {
				got, err := androidBuildTool(dir, tool, goos)
				require.NoError(t, err, tool)
				assert.Equal(t, filepath.Join(dir, files[i]), got)
			}
		})
	}

	// The other OS's layout is not found: the bare name is not on disk on
	// Windows, and a .bat is not what a Unix SDK ships.
	win := fakeBuildTools(t, sdkLayouts["windows"])
	_, err := androidBuildTool(win, "d8", "linux")
	assert.ErrorContains(t, err, "d8 not found in Android SDK build-tools")
	unix := fakeBuildTools(t, sdkLayouts["linux"])
	_, err = androidBuildTool(unix, "d8", "windows")
	assert.ErrorContains(t, err, "d8 not found in Android SDK build-tools")
}

func TestBuildToolFileUnknownPanics(t *testing.T) {
	assert.Panics(t, func() { BuildToolFile("javac") })
}

// The exported lookups resolve under ANDROID_HOME with the host's names.
func TestAndroidBuildToolHost(t *testing.T) {
	goos := runtime.GOOS
	if _, ok := sdkLayouts[goos]; !ok {
		goos = "linux"
	}
	home := t.TempDir()
	dir := filepath.Join(home, "build-tools", "35.0.0")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	for _, f := range sdkLayouts[goos] {
		require.NoError(t, os.WriteFile(filepath.Join(dir, f), nil, 0o755))
	}
	t.Setenv("ANDROID_HOME", home)

	d8, err := AndroidBuildTool("d8")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, BuildToolFile("d8")), d8)
	aapt2, err := Aapt2Path()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, BuildToolFile("aapt2")), aapt2)
}

// An SDK whose build-tools has no version subdirectory is recognised by its
// zipalign, which is zipalign.exe on Windows.
func TestAndroidBuildToolsPathFlat(t *testing.T) {
	for _, zipalign := range []string{"zipalign", "zipalign.exe"} {
		home := t.TempDir()
		dir := filepath.Join(home, "build-tools")
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "lib"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, zipalign), nil, 0o755))
		t.Setenv("ANDROID_HOME", home)
		assert.Equal(t, dir, AndroidBuildToolsPath(), zipalign)
	}
}

func TestBatchCmdLine(t *testing.T) {
	bat := `C:\Program Files\Android\build-tools\35.0.0\d8.bat`
	got, err := batchCmdLine(bat, []string{"--output", `C:\my app & co\out`, `C:\dir\`, "a=b;c,d", "(x) <y> |z|", ""})
	require.NoError(t, err)
	assert.Equal(t, `/d /v:off /s /c ""C:\Program Files\Android\build-tools\35.0.0\d8.bat" "--output" "C:\my app & co\out" "C:\dir\\" "a=b;c,d" "(x) <y> |z|" """`, got)

	for _, a := range []string{"100%", "%PATH%", "wow!", "a^b", `say "hi"`, "two\nlines", "cr\r"} {
		_, err := batchCmdLine(bat, []string{"sign", a})
		assert.ErrorContains(t, err, "cannot pass", a)
	}
	_, err = batchCmdLine(`C:\50%\d8.bat`, nil)
	assert.Error(t, err, "the batch file's own path is checked too")
}

func TestCommandRefusesUnsafeBatchArgs(t *testing.T) {
	_, err := Command(filepath.Join(t.TempDir(), "apksigner.bat"), "--ks-pass", "pass:100%")
	assert.ErrorContains(t, err, "cannot pass")
	_, err = Command(filepath.Join(t.TempDir(), "APKSIGNER.CMD"), "a^b")
	assert.ErrorContains(t, err, "cannot pass")
	cmd, err := Command(filepath.Join(t.TempDir(), "zipalign"), "100%")
	require.NoError(t, err, "only batch files are checked")
	assert.Equal(t, "100%", cmd.Args[1])
}

// On Windows, a batch file run through Command gets every argument intact,
// relayed the way the build-tools wrappers relay them to java: each %1 is
// appended to a list that a CALL then passes on. The test binary stands in
// for java and writes back the arguments it received.
func TestCommandRunsBatchFile(t *testing.T) {
	if os.Getenv("BELLUI_BATCH_ECHO") != "" {
		return
	}
	if runtime.GOOS != "windows" {
		t.Skip("batch files run through cmd.exe only on Windows")
	}
	dir := filepath.Join(t.TempDir(), "build tools & co")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	bat := filepath.Join(dir, "d8.bat")
	require.NoError(t, os.WriteFile(bat, []byte(strings.Join([]string{
		"@echo off",
		"setlocal",
		"set params=",
		":loop",
		"if [%1]==[] goto end",
		"set params=%params% %1",
		"shift",
		"goto loop",
		":end",
		`call "%BELLUI_ECHO_EXE%" -test.run=TestBatchEcho -- %params%`,
		"",
	}, "\r\n")), 0o644))

	exe, err := os.Executable()
	require.NoError(t, err)
	want := []string{"--output", `C:\my app & co\out`, `C:\dir\`, "a=b;c,d", "(x) <y> |z|", "plain"}
	cmd, err := Command(bat, want...)
	require.NoError(t, err)
	cmd.Env = append(os.Environ(), "BELLUI_ECHO_EXE="+exe, "BELLUI_BATCH_ECHO=1")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	_, got, ok := strings.Cut(strings.ReplaceAll(string(out), "\r\n", "\n"), "ARGS\n")
	require.True(t, ok, string(out))
	assert.Equal(t, want, strings.Split(strings.TrimSuffix(got, "\nEND\n"), "\n"))
}

// TestBatchEcho is the stand-in program TestCommandRunsBatchFile runs.
func TestBatchEcho(t *testing.T) {
	if os.Getenv("BELLUI_BATCH_ECHO") == "" {
		t.Skip("helper process for TestCommandRunsBatchFile")
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	os.Stdout.WriteString("ARGS\n" + strings.Join(args, "\n") + "\nEND\n")
	os.Exit(0)
}
