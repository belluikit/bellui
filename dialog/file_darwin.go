//go:build !ios && !android && !wasm && !js

package dialog

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"
)

func getFavoriteLocation(homeURI bellui.URI, name string) (bellui.URI, error) {
	return storage.Child(homeURI, name)
}
