//go:build !windows && !android && !ios && !wasm && !js

package dialog

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/theme"
)

func (f *fileDialog) getPlaces() []favoriteItem {
	lister, err := storage.ListerForURI(storage.NewFileURI("/"))
	if err != nil {
		bellui.LogError("could not create lister for /", err)
		return []favoriteItem{}
	}
	return []favoriteItem{{
		"Computer",
		theme.ComputerIcon(),
		lister,
	}}
}

func hideFile(_ string) error {
	return nil
}
