//go:build ios

package mobile

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation

#import <stdlib.h>
#import <stdbool.h>

void iosDeletePath(const char* path);
bool iosExistsPath(const char* path);
void* iosParseUrl(const char* url);
const void* iosReadFromURL(void* url, int* len);

const void* iosOpenFileWriter(void* url, bool truncate, char** errMsg);
void iosCloseFileWriter(void* handle);
const int iosWriteToFile(void* handle, const void* bytes, int len);
void iosReleaseObject(void* ptr);
*/
import "C"

import (
	"errors"
	"io"
	"unsafe"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage/repository"
)

type secureReadCloser struct {
	url    unsafe.Pointer
	closer func()

	data   []byte
	offset int
}

// Declare conformity to ReadCloser interface
var _ io.ReadCloser = (*secureReadCloser)(nil)

func (s *secureReadCloser) Read(p []byte) (int, error) {
	if s.data == nil {
		var length C.int
		s.data = C.GoBytes(C.iosReadFromURL(s.url, &length), length)
	}

	count := len(p)
	remain := len(s.data) - s.offset
	var err error
	if count >= remain {
		count = remain
		err = io.EOF
	}

	newOffset := s.offset + count

	o := 0
	for i := s.offset; i < newOffset; i++ {
		p[o] = s.data[i]
		o++
	}
	s.offset = newOffset
	return count, err
}

func (s *secureReadCloser) Close() error {
	if s.closer != nil {
		s.closer()
		s.closer = nil
	}
	if s.url != nil {
		C.iosReleaseObject(s.url)
		s.url = nil
	}
	return nil
}

type secureWriteCloser struct {
	handle unsafe.Pointer
	closer func()

	offset int
}

// Declare conformity to WriteCloser interface
var _ io.WriteCloser = (*secureWriteCloser)(nil)

func (s *secureWriteCloser) Write(p []byte) (int, error) {
	cb := C.CBytes(p)
	defer C.free(cb)
	count := int(C.iosWriteToFile(s.handle, cb, C.int(len(p))))
	s.offset += count

	return count, nil
}

func (s *secureWriteCloser) Close() error {
	// Flush and close the file before the closer drops the security scope —
	// writing to a URL whose scoped access has ended is not guaranteed to land.
	if s.handle != nil {
		C.iosCloseFileWriter(s.handle)
		s.handle = nil
	}
	if s.closer != nil {
		s.closer()
		s.closer = nil
	}
	return nil
}

func deleteURI(u bellui.URI) error {
	if u.Scheme() != "file" {
		return errors.New("cannot delete from " + u.Scheme() + " scheme on iOS")
	}

	cStr := C.CString(u.Path())
	defer C.free(unsafe.Pointer(cStr))

	C.iosDeletePath(cStr)
	return nil
}

func existsURI(u bellui.URI) (bool, error) {
	if u.Scheme() != "file" {
		return true, errors.New("cannot check existence of " + u.Scheme() + " scheme on iOS")
	}

	cStr := C.CString(u.Path())
	defer C.free(unsafe.Pointer(cStr))

	exists := C.iosExistsPath(cStr)
	return bool(exists), nil
}

func nativeFileOpen(f *fileOpen) (io.ReadCloser, error) {
	if f.uri == nil || f.uri.String() == "" {
		return nil, nil
	}

	cStr := C.CString(f.uri.String())
	defer C.free(unsafe.Pointer(cStr))

	url := C.iosParseUrl(cStr)

	fileStruct := &secureReadCloser{url: url, closer: f.done}
	return fileStruct, nil
}

func nativeFileSave(f *fileSave, truncate bool) (io.WriteCloser, error) {
	if f.uri == nil || f.uri.String() == "" {
		return nil, nil
	}

	cStr := C.CString(f.uri.String())
	defer C.free(unsafe.Pointer(cStr))

	url := C.iosParseUrl(cStr)

	var errMsg *C.char
	handle := C.iosOpenFileWriter(url, C.bool(truncate), &errMsg)
	C.iosReleaseObject(url) // the writer keeps the handle, not the URL
	if handle == nil {
		// fileWriterForURI runs f.done when no writer is produced — running
		// it here as well would end the security scope twice.
		msg := "unable to open file for writing on iOS"
		if errMsg != nil {
			msg += ": " + C.GoString(errMsg)
			C.free(unsafe.Pointer(errMsg))
		}
		return nil, errors.New(msg)
	}
	fileStruct := &secureWriteCloser{handle: handle, closer: f.done}
	return fileStruct, nil
}

func registerRepository(d *driver) {
	repo := &mobileFileRepo{}
	repository.Register("file", repo)
}
