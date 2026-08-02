//go:build wasm

package app

import (
	"github.com/belluikit/bellui"
	intRepo "github.com/belluikit/bellui/internal/repository"
	"github.com/belluikit/bellui/storage/repository"
)

// registerRepositories lives in its own file, tagged plain `wasm`, unlike the
// rest of app_wasm.go (`!ci && wasm`): wasmbrowsertest runs the ci-tagged
// wasm tests in a real Chromium with a real IndexedDB, so the idbfile scheme
// must register there too — stubbing it under ci would leave the storage
// tests asserting against nothing (#79). Its !wasm counterpart is
// repository_other.go; together the two cover `wasm && ci` per the CLAUDE.md
// wasm-tag invariant (#73, #77).
func (a *belluiApp) registerRepositories() {
	repo, err := intRepo.NewIndexDBRepository()
	if err != nil {
		bellui.LogError("failed to create repository: %v", err)
		return
	}

	repository.Register("idbfile", repo)
}
