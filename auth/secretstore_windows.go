package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// platformSecretStore returns a DPAPI-backed store. DPAPI is always available
// on Windows, so this never returns nil on a healthy system.
func platformSecretStore() secretStore {
	return dpapiStore{}
}

// dpapiStore protects the master secret with the Windows Data Protection API
// (CryptProtectData), which encrypts under a key derived from the user's logon
// credential. The ciphertext blob is written to a file next to the token files;
// unlike the plaintext machine.key it cannot be read without the user's DPAPI
// master key, which the OS keeps credential/ACL gated.
type dpapiStore struct{}

func (dpapiStore) path(account string) (string, error) {
	dir, err := tokenDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, secretService+"-"+account+".dpapi"), nil
}

func (s dpapiStore) get(account string) ([]byte, error) {
	path, err := s.path(account)
	if err != nil {
		return nil, err
	}
	blob, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errSecretNotFound
		}
		return nil, err
	}
	return dpapiUnprotect(blob)
}

func (s dpapiStore) set(account string, secret []byte) error {
	path, err := s.path(account)
	if err != nil {
		return err
	}
	blob, err := dpapiProtect(secret)
	if err != nil {
		return err
	}
	return os.WriteFile(path, blob, 0o600)
}

func (s dpapiStore) delete(account string) error {
	path, err := s.path(account)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

var (
	crypt32         = windows.NewLazySystemDLL("crypt32.dll")
	kernel32        = windows.NewLazySystemDLL("kernel32.dll")
	procProtectData = crypt32.NewProc("CryptProtectData")
	procUnprotect   = crypt32.NewProc("CryptUnprotectData")
	procLocalFree   = kernel32.NewProc("LocalFree")
)

// cryptProtectUIForbidden disables any DPAPI UI prompt; failures return an error
// instead of blocking on a dialog.
const cryptProtectUIForbidden = 0x1

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func newBlob(d []byte) dataBlob {
	if len(d) == 0 {
		return dataBlob{}
	}
	return dataBlob{cbData: uint32(len(d)), pbData: &d[0]}
}

func (b dataBlob) bytes() []byte {
	out := make([]byte, b.cbData)
	copy(out, unsafe.Slice(b.pbData, b.cbData))
	return out
}

func dpapiProtect(data []byte) ([]byte, error) {
	in := newBlob(data)
	var out dataBlob
	r, _, err := procProtectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0,
		cryptProtectUIForbidden,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, fmt.Errorf("auth dpapi protect: %w", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return out.bytes(), nil
}

func dpapiUnprotect(data []byte) ([]byte, error) {
	in := newBlob(data)
	var out dataBlob
	r, _, err := procUnprotect.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0,
		cryptProtectUIForbidden,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, fmt.Errorf("auth dpapi unprotect: %w", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return out.bytes(), nil
}
