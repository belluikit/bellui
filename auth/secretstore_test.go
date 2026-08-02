package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMasterSecret_UsesStoreAndPersists(t *testing.T) {
	dir := redirectStorage(t)
	store := newFakeStore()
	useStore(t, store)

	s1, err := masterSecret()
	require.NoError(t, err)
	assert.Len(t, s1, 32)

	// Persisted in the store, not as a machine.key file.
	assert.Equal(t, s1, store.data[masterSecretAccount])
	_, statErr := os.Stat(filepath.Join(dir, "bellui-auth", "machine.key"))
	assert.True(t, os.IsNotExist(statErr), "should not write file fallback when a store is present")

	// A second call returns the same stored secret.
	s2, err := masterSecret()
	require.NoError(t, err)
	assert.Equal(t, s1, s2)
}

func TestSaveLoadToken_RoundTripViaStore(t *testing.T) {
	redirectStorage(t)
	useStore(t, newFakeStore())

	require.NoError(t, SaveToken("google", "refresh-xyz", "user@example.com"))

	refresh, email, err := LoadToken("google")
	require.NoError(t, err)
	assert.Equal(t, "refresh-xyz", refresh)
	assert.Equal(t, "user@example.com", email)
}

func TestEncodeDecodeSecret_RoundTrip(t *testing.T) {
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i)
	}
	decoded, err := decodeSecret(encodeSecret(secret))
	require.NoError(t, err)
	assert.Equal(t, secret, decoded)
}

func TestDecodeSecret_Invalid(t *testing.T) {
	_, err := decodeSecret("not-hex!!")
	assert.Error(t, err)
}
