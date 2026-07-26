//go:build mobile

package app

import (
	"path/filepath"

	"github.com/belluikit/bellui/internal/app"
)

// storagePath returns the location of the preferences storage (PXF format).
func (p *preferences) storagePath() string {
	return filepath.Join(p.app.storageRoot(), "preferences.pxf")
}

// storageRoot returns the location of the app storage
func (a *belluiApp) storageRoot() string {
	return filepath.Join(app.RootConfigDir(), a.UniqueID())
}

func (p *preferences) watch() {
	// no-op as we are in mobile simulation mode
}
