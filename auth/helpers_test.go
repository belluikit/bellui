// Shared test plumbing, deliberately untagged: the wasm leg's surviving
// secret-store tests use it too (wasmbrowsertest's filesystem shim supports
// t.TempDir-scoped files; #79 Phase B).

package auth

import (
	"fmt"
	"testing"
)

// redirectStorage points token storage at a temp dir for the duration of a test
// and disables the OS secret store so tests are hermetic (they never touch the
// real Keychain/DPAPI/keyring) and exercise the file-based fallback by default.
func redirectStorage(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig := userConfigDir
	userConfigDir = func() (string, error) { return dir, nil }
	origStore := newSecretStore
	newSecretStore = func() secretStore { return nil }
	origAppID := currentAppID
	currentAppID = func() string { return "com.example.test" }
	t.Cleanup(func() {
		userConfigDir = orig
		newSecretStore = origStore
		currentAppID = origAppID
	})
	return dir
}

func newFakeStore() *fakeSecretStore {
	return &fakeSecretStore{data: map[string][]byte{}}
}

type fakeSecretStore struct {
	data    map[string][]byte
	failGet bool // get returns a non-not-found error (store present but broken)
	failSet bool
}

func (f *fakeSecretStore) get(account string) ([]byte, error) {
	if f.failGet {
		return nil, fmt.Errorf("fake store unavailable")
	}
	v, ok := f.data[account]
	if !ok {
		return nil, errSecretNotFound
	}
	return v, nil
}

func (f *fakeSecretStore) set(account string, secret []byte) error {
	if f.failSet {
		return fmt.Errorf("fake store read-only")
	}
	f.data[account] = append([]byte(nil), secret...)
	return nil
}

func (f *fakeSecretStore) delete(account string) error {
	delete(f.data, account)
	return nil
}

// useStore installs a fake secret store for the test.
func useStore(t *testing.T, store secretStore) {
	t.Helper()
	orig := newSecretStore
	newSecretStore = func() secretStore { return store }
	t.Cleanup(func() { newSecretStore = orig })
}
