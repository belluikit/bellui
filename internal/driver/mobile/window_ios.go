//go:build ios

package mobile

import (
	belluiDriver "github.com/belluikit/bellui/driver"
)

// Assert we are satisfying the driver.NativeWindow interface
var _ belluiDriver.NativeWindow = (*window)(nil)

func (w *window) RunNative(fn func(context any)) {
	fn(&belluiDriver.UnknownContext{})
}
