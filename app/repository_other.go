//go:build (ci && !wasm) || (!ios && !android && !linux && !darwin && !windows && !freebsd && !openbsd && !netbsd && !wasm) || ((tamago || noos || tinygo) && !wasm)

package app

// registerRepositories carries app_other.go's no-op, minus wasm: the wasm
// build registers the IndexedDB repository even under ci (repository_wasm.go,
// #79 — wasmbrowsertest runs a real browser), so every disjunct of
// app_other.go's tag is intersected with !wasm here to keep the pair
// exhaustive and disjoint (#73's coverage rule).
func (a *belluiApp) registerRepositories() {
	// no-op
}
