//go:build ios

package app

import (
	"path/filepath"

	"github.com/belluikit/bellui/internal/app"
)
import "C"

// storagePath returns the location of the preferences storage (PXF format).
func (p *preferences) storagePath() string {
	ret := filepath.Join(p.app.storageRoot(), "preferences.pxf")
	return ret
}

// storageRoot returns the location of the app storage
func (a *belluiApp) storageRoot() string {
	return app.RootConfigDir() // we are in a sandbox, so no app ID added to this path
}

func (p *preferences) watch() {
	// no-op on mobile
}
