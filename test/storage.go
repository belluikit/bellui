package test

import (
	"os"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal"
	"github.com/belluikit/bellui/storage"
)

type testStorage struct {
	*internal.Docs
}

func (s *testStorage) RootURI() bellui.URI {
	return storage.NewFileURI(os.TempDir())
}

func (s *testStorage) docRootURI() (bellui.URI, error) {
	return storage.Child(s.RootURI(), "Documents")
}
