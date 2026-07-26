package test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"
)

var errUnsupportedURLProtocol = errors.New("unsupported URL protocol")

type file struct {
	*os.File
	path string
}

type directory struct {
	bellui.URI
}

// Declare conformity to the ListableURI interface
var _ bellui.ListableURI = (*directory)(nil)

func (f *file) Open() (io.ReadCloser, error) {
	return os.Open(f.path)
}

func (f *file) Save() (io.WriteCloser, error) {
	return os.Open(f.path)
}

func (f *file) ReadOnly() bool {
	return true
}

func (f *file) Name() string {
	return filepath.Base(f.path)
}

func (f *file) URI() bellui.URI {
	return storage.NewFileURI(f.path)
}

func openFile(uri bellui.URI, create bool) (*file, error) {
	if uri.Scheme() != "file" {
		return nil, errUnsupportedURLProtocol
	}

	path := uri.Path()
	if create {
		f, err := os.Create(path)
		return &file{File: f, path: path}, err
	}

	f, err := os.Open(path)
	return &file{File: f, path: path}, err
}

func (d *driver) FileReaderForURI(uri bellui.URI) (bellui.URIReadCloser, error) {
	return openFile(uri, false)
}

func (d *driver) FileWriterForURI(uri bellui.URI) (bellui.URIWriteCloser, error) {
	return openFile(uri, true)
}

func (d *driver) ListerForURI(uri bellui.URI) (bellui.ListableURI, error) {
	if uri.Scheme() != "file" {
		return nil, errUnsupportedURLProtocol
	}

	path := uri.Path()
	s, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !s.IsDir() {
		return nil, fmt.Errorf("path '%s' is not a directory, cannot convert to listable URI", path)
	}

	return &directory{URI: uri}, nil
}

func (d *directory) List() ([]bellui.URI, error) {
	if d.Scheme() != "file" {
		return nil, errUnsupportedURLProtocol
	}

	path := d.Path()
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	urilist := make([]bellui.URI, len(files))
	for i, f := range files {
		urilist[i] = storage.NewFileURI(filepath.Join(path, f.Name()))
	}

	return urilist, nil
}
