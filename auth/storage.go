package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// storedToken is the serialized form of a refresh token.
type storedToken struct {
	Provider     string `json:"provider"`
	RefreshToken string `json:"refresh_token"`
	Email        string `json:"email"`
}

// SaveToken encrypts and persists a refresh token for a provider.
func SaveToken(provider, refreshToken, email string) error {
	dir, err := tokenDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	st := storedToken{
		Provider:     provider,
		RefreshToken: refreshToken,
		Email:        email,
	}
	plaintext, err := json.Marshal(st)
	if err != nil {
		return err
	}

	key, err := deriveKey(provider)
	if err != nil {
		return err
	}
	ciphertext, err := encrypt(plaintext, key)
	if err != nil {
		return err
	}

	path := filepath.Join(dir, provider+".enc")
	return os.WriteFile(path, ciphertext, 0o600)
}

// LoadToken reads and decrypts a stored refresh token for a provider.
// Returns empty strings if no token is stored.
func LoadToken(provider string) (refreshToken, email string, err error) {
	dir, err := tokenDir()
	if err != nil {
		return "", "", err
	}

	path := filepath.Join(dir, provider+".enc")
	ciphertext, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", nil // no stored token
		}
		return "", "", err
	}

	key, err := deriveKey(provider)
	if err != nil {
		return "", "", err
	}
	plaintext, err := decrypt(ciphertext, key)
	if err != nil {
		// Corrupted or wrong key — remove the file.
		os.Remove(path)
		return "", "", nil
	}

	var st storedToken
	if err := json.Unmarshal(plaintext, &st); err != nil {
		os.Remove(path)
		return "", "", nil
	}

	return st.RefreshToken, st.Email, nil
}

// ClearToken removes a stored token for a provider.
func ClearToken(provider string) error {
	dir, err := tokenDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, provider+".enc")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// userConfigDir is the source of the per-user config directory. It is a
// package variable so tests can redirect token storage to a temp dir.
var userConfigDir = os.UserConfigDir

// tokenDir returns the directory for storing encrypted tokens.
func tokenDir() (string, error) {
	configDir, err := userConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(configDir, "bellui-auth"), nil
}

// deriveKey produces a 32-byte AES key for a provider, bound to a per-machine
// master secret (see masterSecret).
//
// Security model: the master secret is held in the OS secret store
// (Keychain/DPAPI/libsecret), which is process/credential/ACL gated by the OS,
// so reading the *.enc token files alone is not enough to decrypt them. On
// systems with no secret store (headless CI, minimal installs, wasm) the master
// secret falls back to the random, 0600 file-based machineKey — obfuscation /
// defence in depth, where protection reduces to the file permissions. Either
// way the secret is machine-local, so *.enc files do not transfer between
// machines. See [masterSecret] and [secretStore].
func deriveKey(provider string) ([]byte, error) {
	secret, err := masterSecret()
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	h.Write([]byte("bellui-auth-v2:"))
	h.Write([]byte(provider))
	h.Write([]byte(":"))
	h.Write(secret)
	return h.Sum(nil), nil
}

// machineKey returns a persistent, randomly-generated 32-byte secret unique to
// this machine and user. It is created on first use and stored with 0600
// permissions alongside the token files. Because it is random — not derived
// from guessable machine attributes — it is not recomputable by an attacker
// who merely knows the host details.
//
// This is the fallback master secret for systems with no OS secret store; the
// preferred source is [masterSecret] via the OS keyring.
func machineKey() ([]byte, error) {
	dir, err := tokenDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "machine.key")

	// O_EXCL so concurrent first-time writers don't clobber each other: the
	// loser reads the winner's key rather than persisting a divergent one.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil, rerr
			}
			if len(data) != 32 {
				return nil, fmt.Errorf("auth: corrupt machine key")
			}
			return data, nil
		}
		return nil, err
	}
	defer f.Close()

	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	if _, err := f.Write(key); err != nil {
		return nil, err
	}
	return key, nil
}

// AES-256-GCM encryption.

func encrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decrypt(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
