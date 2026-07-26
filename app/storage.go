package app

import (
	"os"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal"
	"github.com/belluikit/bellui/storage"
)

type store struct {
	*internal.Docs
	a *belluiApp
}

func (s *store) RootURI() bellui.URI {
	if s.a.UniqueID() == "" {
		bellui.LogError("Storage API requires a unique ID, use app.NewWithID()", nil)
		return storage.NewFileURI(os.TempDir())
	}

	u, err := storage.ParseURI(s.a.storageRoot())
	if err == nil {
		return u
	}
	return storage.NewFileURI(s.a.storageRoot())
}

func (s *store) docRootURI() (bellui.URI, error) {
	return storage.Child(s.RootURI(), "Documents")
}
