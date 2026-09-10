//go:build android

package mobile

/*
#cgo LDFLAGS: -landroid -llog -lEGL -lGLESv2

#include <stdbool.h>
#include <stdlib.h>

bool canListURI(uintptr_t jni_env, uintptr_t ctx, char* uriCstr);
bool createListableURI(uintptr_t jni_env, uintptr_t ctx, char* uriCstr);
char *listURI(uintptr_t jni_env, uintptr_t ctx, char* uriCstr);
char *childContentURI(uintptr_t jni_env, uintptr_t ctx, char* uriCstr, char* nameCstr);
char *createChildContentURI(uintptr_t jni_env, uintptr_t ctx, char* uriCstr, char* mimeCstr, char* nameCstr);
*/
import "C"

import (
	"errors"
	"strings"
	"unsafe"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/driver/mobile/app"
	"github.com/belluikit/bellui/storage"
)

func canListURI(uri bellui.URI) bool {
	uriStr := C.CString(uri.String())
	defer C.free(unsafe.Pointer(uriStr))
	listable := false

	app.RunOnJVM(func(_, env, ctx uintptr) error {
		listable = bool(C.canListURI(C.uintptr_t(env), C.uintptr_t(ctx), uriStr))
		return nil
	})

	return listable
}

func createListableURI(uri bellui.URI) error {
	uriStr := C.CString(uri.String())
	defer C.free(unsafe.Pointer(uriStr))

	ok := false
	app.RunOnJVM(func(_, env, ctx uintptr) error {
		ok = bool(C.createListableURI(C.uintptr_t(env), C.uintptr_t(ctx), uriStr))
		return nil
	})

	if ok {
		return nil
	}
	return errors.New("failed to create directory")
}

func listURI(uri bellui.URI) ([]bellui.URI, error) {
	uriStr := C.CString(uri.String())
	defer C.free(unsafe.Pointer(uriStr))

	var str *C.char
	app.RunOnJVM(func(_, env, ctx uintptr) error {
		str = C.listURI(C.uintptr_t(env), C.uintptr_t(ctx), uriStr)
		return nil
	})
	defer C.free(unsafe.Pointer(str))

	parts := strings.Split(C.GoString(str), "|")
	var list []bellui.URI
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		// nativeURI so each child carries the content:// Name() override;
		// a plain ParseURI leaves Name() answering the document ID (#315).
		list = append(list, nativeURI(part))
	}
	return list, nil
}

// mimeTypeDir is DocumentsContract.Document.MIME_TYPE_DIR — the MIME type that
// makes createDocument produce a directory rather than a file.
const mimeTypeDir = "vnd.android.document/directory"

// contentChild resolves the child of a SAF tree or document URI by display
// name. SAF document IDs are opaque, so there is no path arithmetic to do: the
// provider is asked which of the directory's children carries that name, and
// a name nothing answers to is [storage.ErrNotExists] rather than a URI that
// looks real and is not (#303). Use [storage.CreateChild] to make one.
func contentChild(parent bellui.URI, name string) (bellui.URI, error) {
	uriStr := C.CString(parent.String())
	defer C.free(unsafe.Pointer(uriStr))
	nameStr := C.CString(name)
	defer C.free(unsafe.Pointer(nameStr))

	var child string
	app.RunOnJVM(func(_, env, ctx uintptr) error {
		ptr := C.childContentURI(C.uintptr_t(env), C.uintptr_t(ctx), uriStr, nameStr)
		if unsafe.Pointer(ptr) == C.NULL {
			return nil
		}
		child = C.GoString(ptr)
		C.free(unsafe.Pointer(ptr))
		return nil
	})

	if child == "" {
		return nil, storage.ErrNotExists
	}
	return nativeURI(child), nil
}

// contentCreateChild asks the provider to create name under parent and returns
// the URI it assigned. It fails when a child of that name is already there —
// providers answer createDocument by inventing "name (1)" instead, and a
// caller that asked for "name" did not ask for that.
func contentCreateChild(parent bellui.URI, name string, listable bool) (bellui.URI, error) {
	mime := "application/octet-stream"
	if listable {
		mime = mimeTypeDir
	}

	uriStr := C.CString(parent.String())
	defer C.free(unsafe.Pointer(uriStr))
	mimeStr := C.CString(mime)
	defer C.free(unsafe.Pointer(mimeStr))
	nameStr := C.CString(name)
	defer C.free(unsafe.Pointer(nameStr))

	var child string
	app.RunOnJVM(func(_, env, ctx uintptr) error {
		ptr := C.createChildContentURI(C.uintptr_t(env), C.uintptr_t(ctx), uriStr, mimeStr, nameStr)
		if unsafe.Pointer(ptr) == C.NULL {
			return nil
		}
		child = C.GoString(ptr)
		C.free(unsafe.Pointer(ptr))
		return nil
	})

	if child == "" {
		return nil, errors.New("failed to create " + name + " under " + parent.String())
	}
	return nativeURI(child), nil
}
