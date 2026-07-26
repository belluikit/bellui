//go:build android || ios || wasm || js

package storage

import "github.com/belluikit/bellui"

func isHidden(_ bellui.URI) bool {
	return false
}
