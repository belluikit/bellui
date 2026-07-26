package auth

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// platformSecretStore returns a Keychain-backed store via the system "security"
// CLI, or nil if the binary is unavailable (e.g. iOS, where it is excluded).
func platformSecretStore() secretStore {
	bin, err := exec.LookPath("security")
	if err != nil {
		return nil
	}
	return keychainStore{bin: bin}
}

// keychainStore stores secrets as generic passwords in the user's login
// Keychain. The secret is hex-encoded because generic-password values are
// strings.
//
// Note: a generic password added this way is readable by any process running as
// the user while the login Keychain is unlocked, so this is comparable to the
// file scheme for a same-user attacker; the win is at-rest encryption when the
// Keychain is locked (logout / sleep) and removal of the plaintext key file.
type keychainStore struct{ bin string }

// keychainNotFound is the substring the security CLI prints (to stderr) when an
// item does not exist; the process also exits non-zero (44).
const keychainNotFound = "could not be found"

func (k keychainStore) get(account string) ([]byte, error) {
	var out, errb bytes.Buffer
	cmd := exec.Command(k.bin, "find-generic-password",
		"-a", account, "-s", secretService, "-w")
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if strings.Contains(errb.String(), keychainNotFound) {
			return nil, errSecretNotFound
		}
		return nil, fmt.Errorf("auth keychain get: %w: %s", err, strings.TrimSpace(errb.String()))
	}
	return decodeSecret(strings.TrimSpace(out.String()))
}

func (k keychainStore) set(account string, secret []byte) error {
	// -U updates the item in place if it already exists.
	var errb bytes.Buffer
	cmd := exec.Command(k.bin, "add-generic-password",
		"-a", account, "-s", secretService, "-w", encodeSecret(secret), "-U")
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("auth keychain set: %w: %s", err, strings.TrimSpace(errb.String()))
	}
	return nil
}

func (k keychainStore) delete(account string) error {
	var errb bytes.Buffer
	cmd := exec.Command(k.bin, "delete-generic-password",
		"-a", account, "-s", secretService)
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if strings.Contains(errb.String(), keychainNotFound) {
			return nil
		}
		return fmt.Errorf("auth keychain delete: %w: %s", err, strings.TrimSpace(errb.String()))
	}
	return nil
}
