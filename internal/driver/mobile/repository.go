//go:build !wasm

package mobile

import (
	"github.com/belluikit/bellui"

	"github.com/belluikit/bellui/storage/repository"
)

// declare conformance with repository types
var (
	_ repository.Repository              = (*mobileFileRepo)(nil)
	_ repository.HierarchicalRepository  = (*mobileFileRepo)(nil)
	_ repository.ListableRepository      = (*mobileFileRepo)(nil)
	_ repository.WritableRepository      = (*mobileFileRepo)(nil)
	_ repository.AppendableRepository    = (*mobileFileRepo)(nil)
	_ repository.ChildCreatingRepository = (*mobileFileRepo)(nil)
)

type mobileFileRepo struct{}

func (m *mobileFileRepo) CanList(u bellui.URI) (bool, error) {
	return canListURI(u), nil
}

func (m *mobileFileRepo) CanRead(u bellui.URI) (bool, error) {
	return true, nil // TODO check a file can be read
}

func (m *mobileFileRepo) CanWrite(u bellui.URI) (bool, error) {
	return true, nil // TODO check a file can be written
}

func (m *mobileFileRepo) Child(u bellui.URI, name string) (bellui.URI, error) {
	if u == nil {
		return nil, repository.ErrOperationNotSupported
	}

	switch u.Scheme() {
	case "file":
		return repository.GenericChild(u, name)
	case "content":
		// Not path arithmetic: SAF document IDs are opaque, so the child is
		// whichever of this directory's documents carries that display name,
		// and there is no URI at all for one that does not exist yet (#303).
		return contentChild(u, name)
	}
	return nil, repository.ErrOperationNotSupported
}

// CreateChild implements [repository.ChildCreatingRepository]. It is the way
// to make something under a folder the user picked on Android: SAF hands back
// the document URI when the document is created, so Child + CreateListable
// cannot get there (#303).
func (m *mobileFileRepo) CreateChild(parent bellui.URI, name string, listable bool) (bellui.URI, error) {
	if parent == nil {
		return nil, repository.ErrOperationNotSupported
	}
	if parent.Scheme() == "content" {
		return contentCreateChild(parent, name, listable)
	}

	// file:// can name a child before it exists, so this is the ordinary
	// two-step — the same one storage.CreateChild would fall back to.
	child, err := m.Child(parent, name)
	if err != nil {
		return nil, err
	}
	if listable {
		if err := m.CreateListable(child); err != nil {
			return nil, err
		}
		return child, nil
	}
	w, err := m.Writer(child)
	if err != nil {
		return nil, err
	}
	return child, w.Close()
}

func (m *mobileFileRepo) CreateListable(u bellui.URI) error {
	return createListableURI(u)
}

func (m *mobileFileRepo) Delete(u bellui.URI) error {
	return deleteURI(u)
}

func (m *mobileFileRepo) Destroy(string) {
}

func (m *mobileFileRepo) Exists(u bellui.URI) (bool, error) {
	return existsURI(u)
}

func (m *mobileFileRepo) List(u bellui.URI) ([]bellui.URI, error) {
	return listURI(u)
}

// Parent stays file:// only. SAF exposes no upward link — a document knows its
// ID and its tree root, not its parent — and the /tree/<root>/document/<id>
// URIs the picker vends carry no traversable path, so there is nothing to
// derive a parent from without assuming a provider's ID format (#303).
func (m *mobileFileRepo) Parent(u bellui.URI) (bellui.URI, error) {
	if u == nil || u.Scheme() != "file" {
		return nil, repository.ErrOperationNotSupported
	}

	return repository.GenericParent(u)
}

func (m *mobileFileRepo) Reader(u bellui.URI) (bellui.URIReadCloser, error) {
	return fileReaderForURI(u, nil)
}

func (m *mobileFileRepo) Writer(u bellui.URI) (bellui.URIWriteCloser, error) {
	return fileWriterForURI(u, true, nil)
}

func (m *mobileFileRepo) Appender(u bellui.URI) (bellui.URIWriteCloser, error) {
	return fileWriterForURI(u, false, nil)
}
