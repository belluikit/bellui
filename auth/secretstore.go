package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// secretStore is an OS-backed secret store gated by the operating system
// (process/credential/ACL bound), used to hold the master secret that the
// per-provider token-encryption keys are derived from.
//
// Backends: macOS Keychain, Windows DPAPI, Linux Secret Service (libsecret).
// When no backend is available (headless boxes, minimal installs, wasm) the
// platform constructor returns nil and callers fall back to the file-based
// machineKey scheme.
type secretStore interface {
	// get returns the secret stored under account, or errSecretNotFound if no
	// secret has been stored yet. Any other error means the store is present
	// but unusable, and callers fall back to the file scheme.
	get(account string) ([]byte, error)
	// set stores secret under account, replacing any existing value.
	set(account string, secret []byte) error
	// delete removes the secret under account. Removing a missing secret is
	// not an error.
	delete(account string) error
}

// errSecretNotFound signals that the store is usable but holds no secret for
// the requested account yet.
var errSecretNotFound = errors.New("auth: secret not found")

// secretService is the service/collection name secrets are filed under.
const secretService = "bellui-auth"

// masterSecretAccount is the account name for the shared master secret.
const masterSecretAccount = "master-secret"

// newSecretStore returns the platform secret store, or nil if none is
// available. It is a package variable so tests can inject a fake store (or nil
// to force the file fallback) without touching the real OS keyring.
var newSecretStore = func() secretStore { return platformSecretStore() }

// masterSecret returns the 32-byte master secret that per-provider keys are
// derived from. It is read from (and on first use generated into) the OS secret
// store. If no store is available, or the store is present but unusable, it
// falls back to the file-based machineKey so that auth keeps working on
// headless or minimal systems.
//
// Switching backends (e.g. a keyring that becomes unavailable) yields a
// different secret, so existing *.enc tokens fail to decrypt and trigger a
// one-time re-auth — the same graceful degradation as the v1->v2 transition.
func masterSecret() ([]byte, error) {
	if store := newSecretStore(); store != nil {
		if secret, ok := masterSecretFromStore(store); ok {
			return secret, nil
		}
	}
	return machineKey()
}

// masterSecretFromStore fetches the master secret from store, generating and
// persisting one on first use. It returns ok=false when the store is unusable,
// signalling the caller to fall back to the file scheme.
func masterSecretFromStore(store secretStore) (secret []byte, ok bool) {
	secret, err := store.get(masterSecretAccount)
	if err == nil && len(secret) == 32 {
		return secret, true
	}
	if err != nil && !errors.Is(err, errSecretNotFound) {
		return nil, false // store present but broken — fall back to file
	}

	// Not found (or a corrupt-length value): generate a fresh secret and persist
	// it. O_EXCL-style race safety is the store's job; a losing concurrent writer
	// simply re-reads on the next call.
	secret = make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, secret); err != nil {
		return nil, false
	}
	if err := store.set(masterSecretAccount, secret); err != nil {
		return nil, false
	}
	return secret, true
}

// encodeSecret/decodeSecret hex-encode the binary secret so it can travel
// through text-only backends (the macOS security CLI and secret-tool both treat
// the value as a string). Unused by the raw-bytes Windows backend.

func encodeSecret(secret []byte) string {
	return hex.EncodeToString(secret)
}

func decodeSecret(s string) ([]byte, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("auth: decode stored secret: %w", err)
	}
	return b, nil
}
