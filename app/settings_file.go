//go:build !wasm && !tamago && !noos && !tinygo

package app

import (
	"os"

	"github.com/belluikit/bellui"
)

func (s *settings) load() {
	if err := s.loadFromFile(s.schema.StoragePath()); err != nil {
		bellui.LogError("Settings load error:", err)
	}

	s.setupTheme()
}

func (s *settings) loadFromFile(path string) error {
	data, err := os.ReadFile(path) // #nosec
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(data) == 0 { // a mid-write truncation can leave an empty file
		return nil
	}
	return s.schema.UnmarshalPXF(data)
}
