//go:build android

package mobile

import (
	belluiDriver "github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/internal/driver/mobile/app"
)

// Assert we are satisfying the driver.NativeWindow interface
var _ belluiDriver.NativeWindow = (*window)(nil)

func (w *window) RunNative(f func(context any)) {
	app.RunOnJVM(func(vm, env, ctx uintptr) error {
		data := &belluiDriver.AndroidWindowContext{
			NativeWindow: w.handle,
			AndroidContext: belluiDriver.AndroidContext{
				VM:  vm,
				Env: env,
				Ctx: ctx,
			},
		}
		f(data)
		return nil
	})
}
