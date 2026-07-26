package app

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/belluikit/bellui"
)

var once sync.Once

func (a *belluiApp) cachedIconPath() string {
	if a.Icon() == nil {
		return ""
	}

	dirPath := filepath.Join(rootCacheDir(), a.UniqueID())
	filePath := filepath.Join(dirPath, "icon.png")
	once.Do(func() {
		err := a.saveIconToCache(dirPath, filePath)
		if err != nil {
			filePath = ""
		}
	})

	return filePath
}

func (a *belluiApp) saveIconToCache(dirPath, filePath string) error {
	err := os.MkdirAll(dirPath, 0o700)
	if err != nil {
		bellui.LogError("Unable to create application cache directory", err)
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		bellui.LogError("Unable to create icon file", err)
		return err
	}

	defer file.Close()

	if icon := a.Icon(); icon != nil {
		_, err = file.Write(icon.Content())
		if err != nil {
			bellui.LogError("Unable to write icon contents", err)
			return err
		}
	}

	return nil
}
