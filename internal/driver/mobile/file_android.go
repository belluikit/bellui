//go:build android && !wasm

package mobile

/*
#cgo LDFLAGS: -landroid -llog

#include <stdlib.h>
#include <stdbool.h>

bool deleteURI(uintptr_t jni_env, uintptr_t ctx, char* uriCstr);
bool existsURI(uintptr_t jni_env, uintptr_t ctx, char* uriCstr);
int openFD(uintptr_t jni_env, uintptr_t ctx, char* uriCstr);
void* openStream(uintptr_t jni_env, uintptr_t ctx, char* uriCstr);
char* readStream(uintptr_t jni_env, uintptr_t ctx, void* stream, int len, int* total);
void* saveStream(uintptr_t jni_env, uintptr_t ctx, char* uriCstr, bool truncate);
void writeStream(uintptr_t jni_env, uintptr_t ctx, void* stream, char* data, int len);
void closeStream(uintptr_t jni_env, uintptr_t ctx, void* stream);
*/
import "C"

import (
	"errors"
	"io"
	"os"
	"sync"
	"unsafe"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/driver/mobile/app"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/storage/repository"
)

type javaStream struct {
	stream unsafe.Pointer // java.io.InputStream
	mu     sync.Mutex
	closed bool
}

// Declare conformity to ReadCloser interface
var _ io.ReadCloser = (*javaStream)(nil)

// errStreamClosed is returned when attempting to use a closed stream
var errStreamClosed = errors.New("stream is closed")

func (s *javaStream) Read(p []byte) (int, error) {
	s.mu.Lock()
	if s.closed || s.stream == nil {
		s.mu.Unlock()
		return 0, errStreamClosed
	}
	stream := s.stream
	s.mu.Unlock()

	count := 0
	err := app.RunOnJVM(func(_, env, ctx uintptr) error {
		cCount := C.int(0)
		cBytes := unsafe.Pointer(C.readStream(C.uintptr_t(env), C.uintptr_t(ctx), stream, C.int(len(p)), &cCount))
		if cCount == -1 || cBytes == nil {
			return io.EOF
		}
		defer C.free(cBytes)
		count = int(cCount)

		bytes := C.GoBytes(cBytes, cCount)
		for i := 0; i < int(count); i++ {
			p[i] = bytes[i]
		}
		return nil
	})

	return int(count), err
}

func (s *javaStream) Close() error {
	s.mu.Lock()
	if s.closed || s.stream == nil {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	stream := s.stream
	s.stream = nil
	s.mu.Unlock()

	app.RunOnJVM(func(_, env, ctx uintptr) error {
		C.closeStream(C.uintptr_t(env), C.uintptr_t(ctx), stream)
		return nil
	})

	return nil
}

func openStream(uri string) unsafe.Pointer {
	uriStr := C.CString(uri)
	defer C.free(unsafe.Pointer(uriStr))

	var stream unsafe.Pointer
	app.RunOnJVM(func(_, env, ctx uintptr) error {
		streamPtr := C.openStream(C.uintptr_t(env), C.uintptr_t(ctx), uriStr)
		if streamPtr == C.NULL {
			return os.ErrNotExist
		}

		stream = unsafe.Pointer(streamPtr)
		return nil
	})
	return stream
}

// openDocumentFile opens uri as a file descriptor, or returns nil when the
// provider will not hand one out.
func openDocumentFile(uri string) *os.File {
	uriStr := C.CString(uri)
	defer C.free(unsafe.Pointer(uriStr))

	fd := -1
	app.RunOnJVM(func(_, env, ctx uintptr) error {
		fd = int(C.openFD(C.uintptr_t(env), C.uintptr_t(ctx), uriStr))
		return nil
	})
	if fd < 0 {
		return nil
	}
	return os.NewFile(uintptr(fd), uri)
}

// forwardFile is a descriptor that cannot seek, a pipe from a provider that
// streams the document, without the Seek an *os.File would offer and fail.
type forwardFile struct{ f *os.File }

func (f forwardFile) Read(p []byte) (int, error) { return f.f.Read(p) }
func (f forwardFile) Close() error               { return f.f.Close() }

// nativeFileOpen reads the document through a file descriptor where the
// provider gives one, so a local document can seek as a file does on the
// desktop. Through the InputStream it could only be read forward, and a
// player found no duration and could not seek in MP3, Ogg, Opus or WAV
// (#848). A provider that gives no descriptor is read through its
// InputStream as before.
func nativeFileOpen(f *fileOpen) (io.ReadCloser, error) {
	if f.uri == nil || f.uri.String() == "" {
		return nil, nil
	}

	if file := openDocumentFile(f.uri.String()); file != nil {
		if _, err := file.Seek(0, io.SeekCurrent); err == nil {
			return file, nil
		}
		return forwardFile{file}, nil
	}

	ret := openStream(f.uri.String())
	if ret == nil {
		return nil, storage.ErrNotExists
	}

	stream := &javaStream{}
	stream.stream = ret
	return stream, nil
}

func saveStream(uri string, truncate bool) unsafe.Pointer {
	uriStr := C.CString(uri)
	defer C.free(unsafe.Pointer(uriStr))

	var stream unsafe.Pointer
	app.RunOnJVM(func(_, env, ctx uintptr) error {
		streamPtr := C.saveStream(C.uintptr_t(env), C.uintptr_t(ctx), uriStr, C.bool(truncate))
		if streamPtr == C.NULL {
			return os.ErrNotExist
		}

		stream = unsafe.Pointer(streamPtr)
		return nil
	})
	return stream
}

func nativeFileSave(f *fileSave, truncate bool) (io.WriteCloser, error) {
	if f.uri == nil || f.uri.String() == "" {
		return nil, nil
	}

	ret := saveStream(f.uri.String(), truncate)
	if ret == nil {
		return nil, storage.ErrNotExists
	}

	stream := &javaStream{}
	stream.stream = ret
	return stream, nil
}

// Declare conformity to WriteCloser interface
var _ io.WriteCloser = (*javaStream)(nil)

func (s *javaStream) Write(p []byte) (int, error) {
	s.mu.Lock()
	if s.closed || s.stream == nil {
		s.mu.Unlock()
		return 0, errStreamClosed
	}
	stream := s.stream
	s.mu.Unlock()

	err := app.RunOnJVM(func(_, env, ctx uintptr) error {
		cb := C.CBytes(p)
		defer C.free(cb)
		C.writeStream(C.uintptr_t(env), C.uintptr_t(ctx), stream, (*C.char)(cb), C.int(len(p)))
		return nil
	})

	return len(p), err
}

func deleteURI(u bellui.URI) error {
	uriStr := C.CString(u.String())
	defer C.free(unsafe.Pointer(uriStr))

	ok := false
	app.RunOnJVM(func(_, env, ctx uintptr) error {
		ok = bool(C.deleteURI(C.uintptr_t(env), C.uintptr_t(ctx), uriStr))
		return nil
	})

	if !ok {
		return errors.New("failed to delete file " + u.String())
	}
	return nil
}

func existsURI(uri bellui.URI) (bool, error) {
	uriStr := C.CString(uri.String())
	defer C.free(unsafe.Pointer(uriStr))

	ok := false
	app.RunOnJVM(func(_, env, ctx uintptr) error {
		ok = bool(C.existsURI(C.uintptr_t(env), C.uintptr_t(ctx), uriStr))
		return nil
	})

	return ok, nil
}

func registerRepository(d *driver) {
	repo := &mobileFileRepo{}
	repository.Register("file", repo)
	repository.Register("content", repo)
}
