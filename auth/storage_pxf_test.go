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

// writeLegacyToken saves a token for provider as a version before #523 did, as
// JSON, and returns its path.
func writeLegacyToken(t *testing.T, provider, refreshToken, email string) string {
	t.Helper()
	path, appID, err := tokenPath(provider, "")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	old, err := json.Marshal(legacyToken{Provider: provider, RefreshToken: refreshToken, Email: email})
	require.NoError(t, err)
	key, err := deriveKey(appID, provider, "")
	require.NoError(t, err)
	ciphertext, err := encrypt(old, key)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, ciphertext, 0o600))
	return path
}

// A token saved as JSON by a version before #523 still loads, and is rewritten
// as PXF.
func TestLoadToken_LegacyJSON(t *testing.T) {
	redirectStorage(t)
	writeLegacyToken(t, "google", "refresh-old", "old@example.com")

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

// A load that rewrites a JSON record must not write it back after a sign-out
// that ran while it loaded: the user would be signed in again at next launch.
func TestLoadToken_LegacyRewriteDoesNotUndoClear(t *testing.T) {
	redirectStorage(t)
	for i := range 200 {
		path := writeLegacyToken(t, "google", "refresh-old", "old@example.com")
		start := make(chan struct{})
		done := make(chan struct{})
		go func() {
			<-start
			_, _, _ = LoadToken("google")
			close(done)
		}()
		close(start)
		require.NoError(t, ClearToken("google"))
		<-done
		_, err := os.Stat(path)
		require.True(t, os.IsNotExist(err), "iteration %d: the token is back after sign-out", i)
	}
}

// A save that fails leaves the token that was saved before, rather than a
// partial file the next load would remove.
func TestSaveToken_FailedWriteKeepsToken(t *testing.T) {
	redirectStorage(t)
	require.NoError(t, SaveToken("google", "refresh-1", "a@example.com"))
	path, _, err := tokenPath("google", "")
	require.NoError(t, err)
	require.NoError(t, os.Mkdir(path+tempSuffix, 0o700)) // the write cannot open its file

	assert.Error(t, SaveToken("google", "refresh-2", "a@example.com"))
	refresh, _, err := LoadToken("google")
	require.NoError(t, err)
	assert.Equal(t, "refresh-1", refresh)
	require.NoError(t, os.Remove(path+tempSuffix))
}

// A write cut short before its rename leaves a file beside the token, which
// still decrypts; signing out removes it with the token.
func TestClearToken_RemovesPartialWrite(t *testing.T) {
	redirectStorage(t)
	require.NoError(t, SaveToken("google", "refresh-1", "a@example.com"))
	path, _, err := tokenPath("google", "")
	require.NoError(t, err)
	ciphertext, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path+tempSuffix, ciphertext, 0o600))

	require.NoError(t, ClearToken("google"))
	_, err = os.Stat(path + tempSuffix)
	assert.True(t, os.IsNotExist(err))
}

// A string that is not valid UTF-8 is saved as JSON saved it. PXF writes it but
// refuses to read it back, and a record that does not read is removed.
func TestSaveToken_InvalidUTF8(t *testing.T) {
	redirectStorage(t)
	require.NoError(t, SaveToken("google", "refresh-1", "caf\xe9@example.com"))
	refresh, email, err := LoadToken("google")
	require.NoError(t, err)
	assert.Equal(t, "refresh-1", refresh)
	assert.Equal(t, "caf\uFFFD@example.com", email)
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
