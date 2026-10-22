//go:build !wasm

package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readPlaintext decrypts the stored record for provider's own token.
func readPlaintext(t *testing.T, provider string) []byte {
	t.Helper()
	path, appID, err := tokenPath(provider, "")
	require.NoError(t, err)
	ciphertext, err := os.ReadFile(path)
	require.NoError(t, err)
	key, err := deriveKey(appID, provider, "")
	require.NoError(t, err)
	plaintext, err := decrypt(ciphertext, key)
	require.NoError(t, err)
	return plaintext
}

// A saved token record is PXF (#523).
func TestSaveToken_WritesPXF(t *testing.T) {
	redirectStorage(t)
	require.NoError(t, SaveToken("google", "refresh-123", "user@example.com"))

	plaintext := readPlaintext(t, "google")
	assert.Contains(t, string(plaintext), `refresh_token = "refresh-123"`)
	_, legacy, err := unmarshalToken(plaintext)
	require.NoError(t, err)
	assert.False(t, legacy)
}

// A token saved as JSON by a version before #523 still loads, and is rewritten
// as PXF.
func TestLoadToken_LegacyJSON(t *testing.T) {
	redirectStorage(t)
	path, appID, err := tokenPath("google", "")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	old, err := json.Marshal(legacyToken{Provider: "google", RefreshToken: "refresh-old", Email: "old@example.com"})
	require.NoError(t, err)
	key, err := deriveKey(appID, "google", "")
	require.NoError(t, err)
	ciphertext, err := encrypt(old, key)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, ciphertext, 0o600))

	refresh, email, err := LoadToken("google")
	require.NoError(t, err)
	assert.Equal(t, "refresh-old", refresh)
	assert.Equal(t, "old@example.com", email)

	plaintext := readPlaintext(t, "google")
	assert.NotEqual(t, byte('{'), plaintext[0], "the record was rewritten as PXF")
	refresh, email, err = LoadToken("google")
	require.NoError(t, err)
	assert.Equal(t, "refresh-old", refresh)
	assert.Equal(t, "old@example.com", email)
}
