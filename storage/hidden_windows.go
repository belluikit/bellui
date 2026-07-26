package storage

import (
	"syscall"

	"github.com/belluikit/bellui"
)

func isHidden(file bellui.URI) bool {
	if file.Scheme() != "file" {
		return false
	}

	point, err := syscall.UTF16PtrFromString(file.Path())
	if err != nil {
		bellui.LogError("Error making string pointer", err)
		return false
	}
	attr, err := syscall.GetFileAttributes(point)
	if err != nil {
		bellui.LogError("Error getting file attributes", err)
		return false
	}

	return attr&syscall.FILE_ATTRIBUTE_HIDDEN != 0
}
