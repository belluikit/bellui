//go:build !ci && !legacy && !wasm

package app

/*
#cgo LDFLAGS: -framework Foundation -framework UserNotifications
*/
import "C"
