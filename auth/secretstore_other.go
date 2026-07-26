//go:build !darwin && !windows && !linux

package auth

// platformSecretStore returns nil on platforms with no supported OS secret
// store (wasm, BSDs, etc.), so callers fall back to the file-based machineKey.
func platformSecretStore() secretStore { return nil }
