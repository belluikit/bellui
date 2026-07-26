//go:build !windows && !android && !ios && !wasm && !js

package storage

import (
	"path/filepath"

	"github.com/belluikit/bellui"
)

func isHidden(file bellui.URI) bool {
	if file.Scheme() != "file" {
		return false
	}

	name := filepath.Base(file.Path())
	return name == "" || name[0] == '.'
}
