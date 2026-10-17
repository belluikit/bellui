// The token store under test is the on-disk one (config-dir files,
// permission bits); the browser has neither.
// (#77's split convention; #79 Phase B.)
//go:build !wasm

package auth

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
)

// realCurrentAppID is the production currentAppID, which redirectStorage
// replaces with a fixed ID.
var realCurrentAppID = currentAppID

// idApp is an app whose only working method is UniqueID.
type idApp struct {
	bellui.App
	id string
}

func (a idApp) UniqueID() string { return a.id }

// useCurrentApp makes a the running app, and the real currentAppID the source
// of the app ID, for the duration of a test.
func useCurrentApp(t *testing.T, a bellui.App) {
	t.Helper()
	prev := bellui.CurrentApp()
	bellui.SetCurrentApp(a)
	currentAppID = realCurrentAppID
	t.Cleanup(func() { bellui.SetCurrentApp(prev) })
}

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

	a, err := deriveKey("com.example.test", "google", "")
	require.NoError(t, err)
	b, err := deriveKey("com.example.test", "github", "")
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
	c, err := deriveKey("com.example.other", "google", "")
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

// An app created without an ID reports a stand-in "missing-id-<time>" that
// changes every launch. A token stored under it could never be read back, so
// it counts as no ID; so does no running app at all.
func TestTokens_NeedARealAppID(t *testing.T) {
	redirectStorage(t)

	useCurrentApp(t, idApp{id: "missing-id-1760600000"})
	assert.ErrorIs(t, SaveToken("google", "t", "e"), ErrNoAppID)

	useCurrentApp(t, nil)
	assert.ErrorIs(t, SaveToken("google", "t", "e"), ErrNoAppID)

	useCurrentApp(t, idApp{id: "com.example.notes"})
	require.NoError(t, SaveToken("google", "t", "e"))
	refresh, _, err := LoadToken("google")
	require.NoError(t, err)
	assert.Equal(t, "t", refresh)
}

func TestStorageName(t *testing.T) {
	for _, plain := range []string{"com.example.notes", "google", "com.example", "a", "my_app-2"} {
		assert.Equal(t, plain, storageName(plain))
	}
	for _, unsafe := range []string{
		"../../escape", "a/b", "..", ".", `c:\x`, "",
		"Com.Example.Notes",                       // case-insensitive file systems would share it with com.example.notes
		"app.",                                    // Windows drops the trailing dot
		"con", "NUL", "aux.app", "com1", "lpt9.x", // Windows device names
		"machine.key", // the fallback master secret's file
	} {
		got := storageName(unsafe)
		assert.Len(t, got, 64, unsafe)
		assert.NotContains(t, got, string(filepath.Separator), unsafe)
		assert.Equal(t, strings.ToLower(got), got, unsafe)
	}
	assert.NotEqual(t, strings.ToLower(storageName("Com.Example.Notes")),
		strings.ToLower(storageName("com.example.notes")))
}

// A provider name cannot place a token outside the app's own directory, where
// it would overwrite, read or (via LoadToken's clean-up) delete another app's.
func TestTokenPath_ProviderStaysInsideAppDir(t *testing.T) {
	dir := redirectStorage(t)
	appDir := filepath.Join(dir, "bellui-auth", "com.example.test")

	for _, provider := range []string{"../com.example.mail/google", "a/b", "..", "Google"} {
		got, _, err := tokenPath(provider, "")
		require.NoError(t, err)
		assert.Equal(t, appDir, filepath.Dir(got), provider)
	}
	got, _, err := tokenPath("google", "")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(appDir, "google.enc"), got)
}

// The app ID and provider do not run together in the key: moving a character
// from one to the other gives a different key.
func TestDeriveKey_FieldsAreSeparate(t *testing.T) {
	redirectStorage(t)

	a, err := deriveKey("a:b", "c", "")
	require.NoError(t, err)
	b, err := deriveKey("a", "b:c", "")
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
}

// Two issuers under one provider name keep separate tokens: two Keycloak
// realms used to share keycloak.enc, so the second sign-in overwrote the
// first (#356).
func TestTokensPerIssuer(t *testing.T) {
	redirectStorage(t)
	const realmA = "https://sso.example.com/realms/a"
	const realmB = "https://sso.example.com/realms/b"

	require.NoError(t, SaveTokenFor("keycloak", realmA, "refresh-a", "a@example.com"))
	require.NoError(t, SaveTokenFor("keycloak", realmB, "refresh-b", "b@example.com"))

	refresh, email, err := LoadTokenFor("keycloak", realmA)
	require.NoError(t, err)
	assert.Equal(t, "refresh-a", refresh)
	assert.Equal(t, "a@example.com", email)
	refresh, _, err = LoadTokenFor("keycloak", realmB)
	require.NoError(t, err)
	assert.Equal(t, "refresh-b", refresh)

	refresh, _, err = LoadToken("keycloak")
	require.NoError(t, err)
	assert.Empty(t, refresh, "the provider-only token is a third, separate one")

	require.NoError(t, ClearTokenFor("keycloak", realmA))
	refresh, _, _ = LoadTokenFor("keycloak", realmA)
	assert.Empty(t, refresh, "signing out of realm a removes its token")
	refresh, _, _ = LoadTokenFor("keycloak", realmB)
	assert.Equal(t, "refresh-b", refresh, "and leaves realm b's")
}

// A token without an issuer keeps the file name and key it had before
// issuers existed, so tokens saved by v1.1 still load.
func TestTokenWithoutIssuerIsUnchanged(t *testing.T) {
	dir := redirectStorage(t)
	appDir := filepath.Join(dir, "bellui-auth", "com.example.test")
	path, _, err := tokenPath("google", "")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(appDir, "google.enc"), path)

	// The key as v1.1 derived it, written out rather than through deriveKey,
	// so a change to deriveKey's two-field form cannot pass unnoticed.
	secret, err := masterSecret()
	require.NoError(t, err)
	h := sha256.New()
	h.Write([]byte("bellui-auth-v3:"))
	for _, field := range []string{"com.example.test", "google"} {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(field)))
		h.Write(n[:])
		h.Write([]byte(field))
	}
	h.Write(secret)
	key, err := deriveKey("com.example.test", "google", "")
	require.NoError(t, err)
	assert.Equal(t, h.Sum(nil), key)

	withIssuer, err := deriveKey("com.example.test", "keycloak", "https://sso.example.com/realms/a")
	require.NoError(t, err)
	withoutIssuer, err := deriveKey("com.example.test", "keycloak", "")
	require.NoError(t, err)
	assert.NotEqual(t, withoutIssuer, withIssuer)

	issuerPath, _, err := tokenPath("keycloak", "https://sso.example.com/realms/a")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(appDir, issuerDir), filepath.Dir(issuerPath),
		"an issuer's token stays in the app's folder")
}

// No two (provider, issuer) pairs share a file, and no issuer's token shares a
// provider's own: LoadTokenFor deletes a file it cannot decrypt, so a shared
// file would let one token's read delete the other.
func TestTokenPath_IssuerNamesDoNotCollide(t *testing.T) {
	redirectStorage(t)
	paths := map[string]string{}
	for _, pair := range [][2]string{
		{"a", "b\x00c"}, {"a\x00b", "c"}, // a bare separator joined these
		{"a", "b"}, {"a\x00b", ""}, // and these, the second a provider's own
		{"issuers", ""}, // a provider named like the issuers folder
		// A provider whose name is the bytes an issuer token's name hashes:
		// only the issuers folder keeps these two apart.
		{string(lengthPrefixed("keycloak", "x")), ""}, {"keycloak", "x"},
	} {
		path, _, err := tokenPath(pair[0], pair[1])
		require.NoError(t, err)
		if prev, ok := paths[path]; ok {
			t.Errorf("%q and %q share %s", prev, pair, path)
		}
		paths[path] = fmt.Sprintf("%q", pair)
	}
}

// A preset issuer keys like no issuer: a button that names Google's issuer and
// one that leaves Issuer empty share the token LoadToken("google") reads. A
// Microsoft tenant is not the preset's issuer, so it gets a token of its own.
func TestTokenFor_PresetIssuerIsTheProvidersToken(t *testing.T) {
	redirectStorage(t)
	google := Providers["google"].Issuer
	const tenant = "https://login.microsoftonline.com/0000-tenant/v2.0"

	require.NoError(t, SaveTokenFor("google", google, "refresh-g", "g@example.com"))
	refresh, _, err := LoadToken("google")
	require.NoError(t, err)
	assert.Equal(t, "refresh-g", refresh)
	refresh, _, err = LoadTokenFor("google", "")
	require.NoError(t, err)
	assert.Equal(t, "refresh-g", refresh)

	require.NoError(t, SaveTokenFor("microsoft", tenant, "refresh-t", "t@example.com"))
	refresh, _, err = LoadToken("microsoft")
	require.NoError(t, err)
	assert.Empty(t, refresh, "a tenant's token is not the provider's")
	refresh, _, err = LoadTokenFor("microsoft", tenant)
	require.NoError(t, err)
	assert.Equal(t, "refresh-t", refresh)

	require.NoError(t, ClearTokenFor("google", google))
	refresh, _, _ = LoadToken("google")
	assert.Empty(t, refresh, "ClearTokenFor with the preset issuer is ClearToken")
}

// Upgrading from v1.1: a token saved under the provider alone stays where
// LoadToken finds it, a new issuer token does not replace it, and ClearToken
// leaves the issuer token alone (an app signs out of an issuer with
// ClearTokenFor).
func TestTokenFor_ProviderTokenFromBeforeIssuers(t *testing.T) {
	redirectStorage(t)
	const realm = "https://sso.example.com/realms/a"
	require.NoError(t, SaveToken("keycloak", "old", "old@example.com"))

	refresh, _, err := LoadTokenFor("keycloak", realm)
	require.NoError(t, err)
	assert.Empty(t, refresh, "no issuer token before the first sign-in to it")

	require.NoError(t, SaveTokenFor("keycloak", realm, "new", "new@example.com"))
	refresh, _, _ = LoadToken("keycloak")
	assert.Equal(t, "old", refresh)

	require.NoError(t, ClearToken("keycloak"))
	refresh, _, _ = LoadTokenFor("keycloak", realm)
	assert.Equal(t, "new", refresh)
}
