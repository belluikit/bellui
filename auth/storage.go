package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/belluikit/bellui"
)

// ErrNoAppID is returned by SaveToken, LoadToken and ClearToken when the
// running app has no unique ID. Tokens are stored per app, so an app that
// signs users in must be created with app.NewWithID (or have an ID in its
// manifest.pxf), as Preferences already require.
var ErrNoAppID = errors.New("auth: storing a sign-in token needs an app ID; create the app with app.NewWithID")

// storedToken is the serialized form of a refresh token.
type storedToken struct {
	Provider     string `json:"provider"`
	RefreshToken string `json:"refresh_token"`
	Email        string `json:"email"`
}

// SaveToken encrypts and persists a refresh token for a provider. Tokens are
// stored per app (by the running app's unique ID), so two apps that sign in
// with the same provider keep separate tokens.
func SaveToken(provider, refreshToken, email string) error {
	path, appID, err := tokenPath(provider)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
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

	key, err := deriveKey(appID, provider)
	if err != nil {
		return err
	}
	ciphertext, err := encrypt(plaintext, key)
	if err != nil {
		return err
	}

	return os.WriteFile(path, ciphertext, 0o600)
}

// LoadToken reads and decrypts the running app's stored refresh token for a
// provider. Returns empty strings if no token is stored.
func LoadToken(provider string) (refreshToken, email string, err error) {
	path, appID, err := tokenPath(provider)
	if err != nil {
		return "", "", err
	}

	ciphertext, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", nil // no stored token
		}
		return "", "", err
	}

	key, err := deriveKey(appID, provider)
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

// ClearToken removes the running app's stored token for a provider.
func ClearToken(provider string) error {
	path, _, err := tokenPath(provider)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// userConfigDir is the source of the per-user config directory. It is a
// package variable so tests can redirect token storage to a temp dir.
var userConfigDir = os.UserConfigDir

// currentAppID returns the running app's unique ID, or "" when there is no app
// or it has no ID. It is a package variable so tests can set an ID.
var currentAppID = func() string {
	return appIDOf(bellui.CurrentApp())
}

// missingIDPrefix starts the stand-in ID an app created without one reports
// from UniqueID (see belluiApp.UniqueID in the app package). The stand-in
// changes on every launch, so a token stored under it could never be read
// back: it counts as no ID.
const missingIDPrefix = "missing-id-"

// appIDOf returns a's unique ID, or "" when a is nil or has no ID of its own.
func appIDOf(a bellui.App) string {
	if a == nil {
		return ""
	}
	id := a.UniqueID()
	if strings.HasPrefix(id, missingIDPrefix) {
		return ""
	}
	return id
}

// tokenDir returns the bellui-auth directory, which holds the machine key and
// one subdirectory of tokens per app.
func tokenDir() (string, error) {
	configDir, err := userConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(configDir, "bellui-auth"), nil
}

// plainName matches names used as a file or directory name as they are. They
// are lower case, so two names that differ only in case cannot share a file on
// a case-insensitive file system (macOS and Windows by default), and they do
// not end in a dot, which Windows drops.
var plainName = regexp.MustCompile(`^[a-z0-9]([a-z0-9._-]*[a-z0-9_-])?$`)

// windowsDeviceName matches the device names Windows reserves in every
// directory, with or without an extension.
var windowsDeviceName = regexp.MustCompile(`^(con|prn|aux|nul|com[0-9]|lpt[0-9])(\.|$)`)

// storageName returns the file or directory name that stores s: s itself when
// it is a plain name, otherwise its SHA-256 in hex. So an app ID or provider
// can never point outside its directory, collide with another by case, or
// name the machine key.
func storageName(s string) string {
	if plainName.MatchString(s) && !windowsDeviceName.MatchString(s) && s != "machine.key" {
		return s
	}
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// appTokenDir returns the running app's token directory and its app ID.
func appTokenDir() (dir, appID string, err error) {
	appID = currentAppID()
	if appID == "" {
		return "", "", ErrNoAppID
	}
	base, err := tokenDir()
	if err != nil {
		return "", "", err
	}
	return filepath.Join(base, storageName(appID)), appID, nil
}

// tokenPath returns the file holding the running app's token for provider,
// and the app's ID.
func tokenPath(provider string) (path, appID string, err error) {
	dir, appID, err := appTokenDir()
	if err != nil {
		return "", "", err
	}
	return filepath.Join(dir, storageName(provider)+".enc"), appID, nil
}

// deriveKey produces a 32-byte AES key for an app and provider, bound to a
// per-machine master secret (see masterSecret).
//
// Security model: the master secret is held in the OS secret store
// (Keychain/DPAPI/libsecret), which is process/credential/ACL gated by the OS,
// so reading the *.enc token files alone is not enough to decrypt them. On
// systems with no secret store (headless CI, minimal installs, wasm) the master
// secret falls back to the random, 0600 file-based machineKey — obfuscation /
// defence in depth, where protection reduces to the file permissions. Either
// way the secret is machine-local, so *.enc files do not transfer between
// machines. See [masterSecret] and [secretStore].
func deriveKey(appID, provider string) ([]byte, error) {
	secret, err := masterSecret()
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	h.Write([]byte("bellui-auth-v3:"))
	// Each field is length-prefixed, so no two (appID, provider) pairs hash
	// the same bytes, whatever characters they contain.
	for _, field := range []string{appID, provider} {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(field)))
		h.Write(n[:])
		h.Write([]byte(field))
	}
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
