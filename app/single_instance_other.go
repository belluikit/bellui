//go:build android || ios || wasm || js || tamago || noos || tinygo || (!linux && !darwin && !freebsd && !openbsd && !netbsd && !windows)

package app

// singleInstanceBind is a no-op on platforms that are inherently single-instance
// (mobile, web) or that have no supported IPC transport: the process always
// reports itself as the primary and no arguments are ever forwarded.
func singleInstanceBind(_ string, _ func(args []string)) (primary bool, closer func(), err error) {
	return true, nil, nil
}
