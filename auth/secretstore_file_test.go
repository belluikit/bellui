// The file-fallback trio asserts the on-disk master-secret path (config-dir
// files, permission bits), which the browser does not have. The in-memory
// store tests stay on the wasm leg.
// (#77's split convention; #79 Phase B.)
//go:build !wasm

package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMasterSecret_FallsBackToFileWhenNoStore(t *testing.T) {
	redirectStorage(t) // installs newSecretStore -> nil

	s, err := masterSecret()
	require.NoError(t, err)

	mk, err := machineKey()
	require.NoError(t, err)
	assert.Equal(t, mk, s, "no store available should use the file machineKey")
}

func TestMasterSecret_FallsBackToFileWhenStoreBroken(t *testing.T) {
	redirectStorage(t)
	store := newFakeStore()
	store.failGet = true
	useStore(t, store)

	s, err := masterSecret()
	require.NoError(t, err)

	mk, err := machineKey()
	require.NoError(t, err)
	assert.Equal(t, mk, s, "a broken store should fall back to the file machineKey")
}

func TestMasterSecret_FallsBackWhenStoreUnwritable(t *testing.T) {
	redirectStorage(t)
	store := newFakeStore()
	store.failSet = true // get says not-found, set fails
	useStore(t, store)

	s, err := masterSecret()
	require.NoError(t, err)

	mk, err := machineKey()
	require.NoError(t, err)
	assert.Equal(t, mk, s)
}
