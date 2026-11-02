// Package apphook tells the packages under the root one whether an app has
// started, without asking [bellui.CurrentApp], which logs an error when none
// has. A theme looked up before the app exists is ordinary enough that it
// should say so plainly rather than as that error (#861). It imports nothing
// from the module, so every package can use it.
package apphook

import "sync/atomic"

var started atomic.Bool

// SetStarted records whether an app is current. The root package calls it
// from SetCurrentApp.
func SetStarted(s bool) { started.Store(s) }

// Started reports whether an app is current.
func Started() bool { return started.Load() }
