//go:build ios && !wasm

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
char* iosURLPath(void* url);

const void* iosOpenFileWriter(void* url, bool truncate, char** errMsg);
void iosCloseFileWriter(void* handle);
const int iosWriteToFile(void* handle, const void* bytes, int len);
void iosReleaseObject(void* ptr);
*/
import "C"

import (
	"errors"
	"io"
	"os"
	"unsafe"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage/repository"
)

// secureReadCloser reads a picked document while its security scope is held.
// It reads through the document's file, so memory stays bounded by the
// caller's buffer however large the document is (#1020). When the file cannot
// be opened by its path, it falls back to reading the whole document into
// memory on first use, as it did before.
type secureReadCloser struct {
	url    unsafe.Pointer
	closer func()

	file *os.File

	data   []byte
	offset int
}

// Declare conformity to ReadCloser interface
var (
	_ io.ReadSeekCloser = (*secureReadCloser)(nil)
	_ io.ReaderAt       = (*secureReadCloser)(nil)
)

// openFile opens the document at the URL's path. The caller holds its
// security scope, which open needs and the open file keeps no longer.
func (s *secureReadCloser) openFile() error {
	cPath := C.iosURLPath(s.url)
	if cPath == nil {
		return errors.New("the picked URL has no file path")
	}
	defer C.free(unsafe.Pointer(cPath))
	f, err := os.Open(C.GoString(cPath))
	if err != nil {
		return err
	}
	s.file = f
	return nil
}

// load reads the whole document on first use, when it could not be opened as
// a file; Read and Seek then serve from memory.
func (s *secureReadCloser) load() {
	if s.data == nil {
		var length C.int
		s.data = C.GoBytes(C.iosReadFromURL(s.url, &length), length)
	}
}

func (s *secureReadCloser) Read(p []byte) (int, error) {
	if s.file != nil {
		return s.file.Read(p)
	}
	s.load()
	n, err := s.ReadAt(p, int64(s.offset))
	s.offset += n
	return n, err
}

// ReadAt reads len(p) bytes at off, without moving the offset Read reads from.
func (s *secureReadCloser) ReadAt(p []byte, off int64) (int, error) {
	if s.file != nil {
		return s.file.ReadAt(p, off)
	}
	s.load()
	if off < 0 {
		return 0, errors.New("read at a negative offset")
	}
	if off >= int64(len(s.data)) {
		return 0, io.EOF
	}
	n := copy(p, s.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// Seek moves within the document, so a picked file seeks as it does on the
// desktop: without it a player found no duration and could not seek in MP3,
// Ogg, Opus or WAV (#848).
func (s *secureReadCloser) Seek(offset int64, whence int) (int64, error) {
	if s.file != nil {
		return s.file.Seek(offset, whence)
	}
	s.load()
	pos, err := seekWithin(int64(s.offset), int64(len(s.data)), offset, whence)
	if err != nil {
		return int64(s.offset), err
	}
	s.offset = int(pos)
	return pos, nil
}

// Close closes the document's file, then ends its security scope.
func (s *secureReadCloser) Close() error {
	var err error
	if s.file != nil {
		err = s.file.Close()
		s.file = nil
	}
	if s.closer != nil {
		s.closer()
		s.closer = nil
	}
	if s.url != nil {
		C.iosReleaseObject(s.url)
		s.url = nil
	}
	return err
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
	if err := fileStruct.openFile(); err != nil {
		bellui.LogError("iOS: reading the picked file into memory, as it cannot be opened by its path", err)
	}
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
