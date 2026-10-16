// The token store under test is the on-disk one (config-dir files,
// permission bits); the browser has neither.
// (#77's split convention; #79 Phase B.)
//go:build !wasm

package auth

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveLoadToken_RoundTrip(t *testing.T) {
	redirectStorage(t)

	require.NoError(t, SaveToken("google", "refresh-123", "user@example.com"))

	refresh, email, err := LoadToken("google")
	require.NoError(t, err)
	assert.Equal(t, "refresh-123", refresh)
	assert.Equal(t, "user@example.com", email)
}

func TestLoadToken_Missing(t *testing.T) {
	redirectStorage(t)

	refresh, email, err := LoadToken("google")
	require.NoError(t, err)
	assert.Empty(t, refresh)
	assert.Empty(t, email)
}

func TestClearToken(t *testing.T) {
	redirectStorage(t)

	require.NoError(t, SaveToken("okta", "tok", "e@x.com"))
	require.NoError(t, ClearToken("okta"))

	refresh, _, err := LoadToken("okta")
	require.NoError(t, err)
	assert.Empty(t, refresh)

	// Clearing a non-existent token is not an error.
	require.NoError(t, ClearToken("okta"))
}

func TestMachineKey_PersistsAndIsSecret(t *testing.T) {
	dir := redirectStorage(t)

	k1, err := machineKey()
	require.NoError(t, err)
	assert.Len(t, k1, 32)

	// A second call returns the same persisted key.
	k2, err := machineKey()
	require.NoError(t, err)
	assert.Equal(t, k1, k2)

	// The key is a real file, not derived from public machine attributes.
	info, err := os.Stat(filepath.Join(dir, "bellui-auth", "machine.key"))
	require.NoError(t, err)
	// Windows doesn't honour Unix permission bits, so only assert there.
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
}

func TestDeriveKey_DiffersPerProvider(t *testing.T) {
	redirectStorage(t)

	a, err := deriveKey("com.example.test", "google")
	require.NoError(t, err)
	b, err := deriveKey("com.example.test", "github")
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
	c, err := deriveKey("com.example.other", "google")
	require.NoError(t, err)
	assert.NotEqual(t, a, c, "the key is per app too")
}

func TestLoadToken_RemovesUndecryptableFile(t *testing.T) {
	dir := redirectStorage(t)

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "bellui-auth", "com.example.test"), 0o700))
	path := filepath.Join(dir, "bellui-auth", "com.example.test", "google.enc")
	require.NoError(t, os.WriteFile(path, []byte("not a valid ciphertext"), 0o600))

	refresh, _, err := LoadToken("google")
	require.NoError(t, err)
	assert.Empty(t, refresh)

	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "corrupt token file should be removed")
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	plaintext := []byte("hello secret")

	ct, err := encrypt(plaintext, key)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ct)

	pt, err := decrypt(ct, key)
	require.NoError(t, err)
	assert.Equal(t, plaintext, pt)
}

// Two apps that sign in with the same provider keep separate tokens: one app
// saving does not overwrite or expose the other's (#335).
func TestSaveToken_IsPerApp(t *testing.T) {
	dir := redirectStorage(t)

	currentAppID = func() string { return "com.example.notes" }
	require.NoError(t, SaveToken("google", "notes-token", "a@example.com"))
	currentAppID = func() string { return "com.example.mail" }
	require.NoError(t, SaveToken("google", "mail-token", "b@example.com"))

	refresh, email, err := LoadToken("google")
	require.NoError(t, err)
	assert.Equal(t, "mail-token", refresh)
	assert.Equal(t, "b@example.com", email)

	currentAppID = func() string { return "com.example.notes" }
	refresh, email, err = LoadToken("google")
	require.NoError(t, err)
	assert.Equal(t, "notes-token", refresh)
	assert.Equal(t, "a@example.com", email)

	_, err = os.Stat(filepath.Join(dir, "bellui-auth", "com.example.notes", "google.enc"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "bellui-auth", "com.example.mail", "google.enc"))
	assert.NoError(t, err)
}

// Tokens left at the old shared location are not read: users sign in again
// once (owner decision on #335).
func TestLoadToken_IgnoresOldSharedLocation(t *testing.T) {
	dir := redirectStorage(t)
	old := filepath.Join(dir, "bellui-auth", "google.enc")
	require.NoError(t, os.MkdirAll(filepath.Dir(old), 0o700))
	require.NoError(t, os.WriteFile(old, []byte("anything"), 0o600))

	refresh, _, err := LoadToken("google")
	require.NoError(t, err)
	assert.Empty(t, refresh)
}

func TestTokens_NeedAnAppID(t *testing.T) {
	redirectStorage(t)
	currentAppID = func() string { return "" }

	assert.ErrorIs(t, SaveToken("google", "t", "e"), ErrNoAppID)
	_, _, err := LoadToken("google")
	assert.ErrorIs(t, err, ErrNoAppID)
	assert.ErrorIs(t, ClearToken("google"), ErrNoAppID)
}

// An app ID that is not a plain file name cannot place tokens outside
// bellui-auth.
func TestAppTokenDir_StaysInsideBelluiAuth(t *testing.T) {
	dir := redirectStorage(t)
	base := filepath.Join(dir, "bellui-auth")

	for _, id := range []string{"../../escape", "a/b", "..", `c:\x`} {
		currentAppID = func() string { return id }
		got, _, err := appTokenDir()
		require.NoError(t, err)
		assert.Equal(t, base, filepath.Dir(got), id)
	}
	currentAppID = func() string { return "com.example.notes" }
	got, _, err := appTokenDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(base, "com.example.notes"), got)
}
