//go:build !ci && (!android || !ios || !mobile) && wasm && !noos && !tinygo

package app

func rootConfigDir() string {
	return "/data/"
}
