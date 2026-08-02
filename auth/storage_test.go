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

	a, err := deriveKey("google")
	require.NoError(t, err)
	b, err := deriveKey("github")
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
}

func TestLoadToken_RemovesUndecryptableFile(t *testing.T) {
	dir := redirectStorage(t)

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "bellui-auth"), 0o700))
	path := filepath.Join(dir, "bellui-auth", "google.enc")
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
