package auth

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// platformSecretStore returns a Secret Service (libsecret) backed store via the
// "secret-tool" CLI, or nil if the binary is unavailable. A present binary that
// fails at runtime (no D-Bus session / locked or absent keyring on a headless
// box) surfaces as a get/set error, and the caller falls back to the file
// scheme — so auth keeps working without a keyring.
func platformSecretStore() secretStore {
	bin, err := exec.LookPath("secret-tool")
	if err != nil {
		return nil
	}
	return secretToolStore{bin: bin}
}

// secretToolStore stores secrets in the desktop keyring via libsecret. Secrets
// are looked up by the (service, account) attribute pair and hex-encoded since
// the value travels through the CLI as text.
type secretToolStore struct{ bin string }

func (s secretToolStore) get(account string) ([]byte, error) {
	var out, errb bytes.Buffer
	cmd := exec.Command(s.bin, "lookup", "service", secretService, "account", account)
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		// secret-tool exits non-zero with empty output when the item is absent;
		// a non-empty stderr means the keyring itself is unavailable.
		if errb.Len() == 0 {
			return nil, errSecretNotFound
		}
		return nil, fmt.Errorf("auth secret-tool get: %w: %s", err, strings.TrimSpace(errb.String()))
	}
	// lookup prints the value with no trailing newline.
	return decodeSecret(strings.TrimSpace(out.String()))
}

func (s secretToolStore) set(account string, secret []byte) error {
	var errb bytes.Buffer
	cmd := exec.Command(s.bin, "store",
		"--label", "bellui auth token key",
		"service", secretService, "account", account)
	cmd.Stdin = strings.NewReader(encodeSecret(secret))
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("auth secret-tool set: %w: %s", err, strings.TrimSpace(errb.String()))
	}
	return nil
}

func (s secretToolStore) delete(account string) error {
	var errb bytes.Buffer
	cmd := exec.Command(s.bin, "clear", "service", secretService, "account", account)
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("auth secret-tool delete: %w: %s", err, strings.TrimSpace(errb.String()))
	}
	return nil
}
