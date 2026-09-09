//go:build ios

package mobile

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation

#import <stdlib.h>
#import <stdbool.h>

bool iosCanList(const char* url);
bool iosCreateListable(const char* url);
char* iosList(const char* url);
*/
import "C"

import (
	"errors"
	"strings"
	"unsafe"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/storage/repository"
)

func canListURI(uri bellui.URI) bool {
	uriStr := C.CString(uri.String())
	defer C.free(unsafe.Pointer(uriStr))

	return bool(C.iosCanList(uriStr))
}

func createListableURI(uri bellui.URI) error {
	uriStr := C.CString(uri.String())
	defer C.free(unsafe.Pointer(uriStr))

	ok := bool(C.iosCreateListable(uriStr))
	if ok {
		return nil
	}
	return errors.New("failed to create directory")
}

func listURI(uri bellui.URI) ([]bellui.URI, error) {
	uriStr := C.CString(uri.String())
	defer C.free(unsafe.Pointer(uriStr))

	str := C.iosList(uriStr)
	parts := strings.Split(C.GoString(str), "|")
	var list []bellui.URI
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		u, _ := storage.ParseURI(part)
		list = append(list, u)
	}
	return list, nil
}

func contentChild(bellui.URI, string) (bellui.URI, error) {
	// content:// is an Android scheme; iOS security-scoped URLs are file://
	return nil, repository.ErrOperationNotSupported
}

func contentCreateChild(bellui.URI, string, bool) (bellui.URI, error) {
	return nil, repository.ErrOperationNotSupported
}
