package mobile

import (
	"github.com/belluikit/bellui"

	"github.com/belluikit/bellui/storage/repository"
)

// declare conformance with repository types
var (
	_ repository.Repository             = (*mobileFileRepo)(nil)
	_ repository.HierarchicalRepository = (*mobileFileRepo)(nil)
	_ repository.ListableRepository     = (*mobileFileRepo)(nil)
	_ repository.WritableRepository     = (*mobileFileRepo)(nil)
	_ repository.AppendableRepository   = (*mobileFileRepo)(nil)
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
	if u == nil || u.Scheme() != "file" {
		return nil, repository.ErrOperationNotSupported
	}

	return repository.GenericChild(u, name)
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

func (m *mobileFileRepo) Parent(u bellui.URI) (bellui.URI, error) {
	if u == nil || u.Scheme() != "file" {
		return nil, repository.ErrOperationNotSupported
	}

	return repository.GenericParent(u)
}

func (m *mobileFileRepo) Reader(u bellui.URI) (bellui.URIReadCloser, error) {
	return fileReaderForURI(u)
}

func (m *mobileFileRepo) Writer(u bellui.URI) (bellui.URIWriteCloser, error) {
	return fileWriterForURI(u, true)
}

func (m *mobileFileRepo) Appender(u bellui.URI) (bellui.URIWriteCloser, error) {
	return fileWriterForURI(u, false)
}
