//go:build !wasm

package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trendvidia/protowire-go/encoding/pxf"

	"github.com/belluikit/bellui/internal/authstore/tokenpb"
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

// TestToken_GoldenDecode guards the token record format (#523): it decodes a
// committed testdata/token.pxf, the plaintext this version encrypts, so a
// change that stops reading tokens saved on users' machines fails here. Do not
// regenerate it to make the test pass; only when the format changes on purpose:
//
//	BELLUI_REGEN_GOLDEN=1 go test -run TestToken_GoldenDecode ./auth/
func TestToken_GoldenDecode(t *testing.T) {
	path := filepath.Join("testdata", "token.pxf")
	if os.Getenv("BELLUI_REGEN_GOLDEN") != "" {
		data, err := pxf.Marshal(&tokenpb.StoredToken{Provider: "google", RefreshToken: "refresh-123", Email: "user@example.com"})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, data, 0o600))
	}

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	st, legacy, err := unmarshalToken(data)
	require.NoError(t, err)
	assert.False(t, legacy)
	assert.Equal(t, "google", st.GetProvider())
	assert.Equal(t, "refresh-123", st.GetRefreshToken())
	assert.Equal(t, "user@example.com", st.GetEmail())
}
