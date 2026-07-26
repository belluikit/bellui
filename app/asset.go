package app

import (
	"sync/atomic"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"
)

var globalBundle atomic.Pointer[storage.AssetBundle]

// SetAssetBundle registers an asset bundle for the current application.
// This allows resources to be loaded by name from an external directory
// instead of being compiled into the binary.
//
// Since: 2.7
func SetAssetBundle(bundle *storage.AssetBundle) {
	globalBundle.Store(bundle)
}

// Asset returns a resource from the registered asset bundle.
// Returns nil if no bundle is registered or the named asset does not exist.
//
// Since: 2.7
func Asset(name string) bellui.Resource {
	b := globalBundle.Load()
	if b == nil {
		return nil
	}
	return b.Resource(name)
}
