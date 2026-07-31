package webview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/belluikit/bellui/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetCEFProfile restores the package-level profile choice after a test.
func resetCEFProfile(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		cefProfile.dir = ""
		cefProfile.inMemory = false
		cefProfile.locked = false
	})
}

func TestCEFProfile_DefaultPersistsUnderAppStorage(t *testing.T) {
	resetCEFProfile(t)
	test.NewTempApp(t) // its storage root is os.TempDir()

	cache, root := resolveCEFProfile()
	assert.Equal(t, filepath.Join(os.TempDir(), "webview-cef"), cache,
		"the default profile lives under the app storage root")
	assert.Equal(t, cache, root, "installation data sits with the profile")
}

func TestCEFProfile_ExplicitDir(t *testing.T) {
	resetCEFProfile(t)
	dir := t.TempDir()

	require.NoError(t, SetCEFProfileDir(dir))
	cache, root := resolveCEFProfile()
	assert.Equal(t, dir, cache)
	assert.Equal(t, dir, root)
}

func TestCEFProfile_RelativeDirBecomesAbsolute(t *testing.T) {
	resetCEFProfile(t)

	require.NoError(t, SetCEFProfileDir("some-profile"))
	cache, root := resolveCEFProfile()
	assert.True(t, filepath.IsAbs(cache), "a relative dir is resolved: %s", cache)
	assert.Equal(t, "some-profile", filepath.Base(cache))
	assert.Equal(t, cache, root)
}

func TestCEFProfile_EmptyDirRestoresDefault(t *testing.T) {
	resetCEFProfile(t)
	test.NewTempApp(t)

	require.NoError(t, SetCEFProfileDir(t.TempDir()))
	require.NoError(t, SetCEFProfileDir(""))
	cache, _ := resolveCEFProfile()
	assert.Equal(t, filepath.Join(os.TempDir(), "webview-cef"), cache)
}

func TestCEFProfile_InMemory(t *testing.T) {
	resetCEFProfile(t)

	require.NoError(t, SetCEFProfileInMemory())
	cache, root := resolveCEFProfile()
	assert.Empty(t, cache, "in-memory means no cache path — incognito")
	require.NotEmpty(t, root, "installation data still needs a home of its own")
	assert.True(t, strings.HasPrefix(filepath.Base(root), "bellui-webview-cef-"),
		"a fresh per-process directory, so instances never contend: %s", root)
	st, err := os.Stat(root)
	require.NoError(t, err)
	assert.True(t, st.IsDir())
	t.Cleanup(func() { os.RemoveAll(root) })
}

func TestCEFProfile_DirAfterInMemoryPersistsAgain(t *testing.T) {
	resetCEFProfile(t)
	dir := t.TempDir()

	require.NoError(t, SetCEFProfileInMemory())
	require.NoError(t, SetCEFProfileDir(dir))
	cache, _ := resolveCEFProfile()
	assert.Equal(t, dir, cache, "the later call wins")
}

func TestCEFProfile_LockedAfterEngineStart(t *testing.T) {
	resetCEFProfile(t)
	dir := t.TempDir()
	require.NoError(t, SetCEFProfileDir(dir))

	cefProfile.locked = true // what the Linux runtime does at first Attach

	assert.Error(t, SetCEFProfileDir(t.TempDir()), "too late: CEF is once-per-process")
	assert.Error(t, SetCEFProfileInMemory())
	cache, root := resolveCEFProfile()
	assert.Equal(t, dir, cache, "the refused calls changed nothing")
	assert.Equal(t, dir, root)
}
