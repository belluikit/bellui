//go:build !android

package mobile

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"
)

func nativeURI(path string) bellui.URI {
	uri, err := storage.ParseURI(path)
	if err != nil {
		bellui.LogError("Error on parsing uri", err)
	}
	return uri
}
