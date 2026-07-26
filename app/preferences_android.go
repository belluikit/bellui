//go:build android

package app

import (
	"path/filepath"

	"github.com/belluikit/bellui/internal/app"
)

// storagePath returns the location of the preferences storage (PXF format).
func (p *preferences) storagePath() string {
	// we have no global storage, use app global instead - rootConfigDir looks up in app_mobile_and.go
	return filepath.Join(p.app.storageRoot(), "preferences.pxf")
}

// storageRoot returns the location of the app storage
func (a *belluiApp) storageRoot() string {
	return app.RootConfigDir() // we are in a sandbox, so no app ID added to this path
}

func (p *preferences) watch() {
	// no-op on mobile
}
