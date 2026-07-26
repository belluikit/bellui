//go:build test

package painter

import "github.com/belluikit/typesetting/fontscan"

// This file is the `test` build-tag counterpart of font_prod.go and
// font_families_prod.go (which are `//go:build !test`). Under `-tags test` the
// host system-font scan is stubbed out so builds and tests are deterministic
// and do not depend on whatever fonts happen to be installed — glyphs resolve
// only from the bundled and app-registered faces.
//
// It is a regular (non-_test.go) build file on purpose, so that `go build`,
// `go vet` and gopls all work under `-tags test`, not only `go test`.
//
// (The former freetype-era glyph/cmap unit tests that once lived alongside the
// loadSystemFonts stub tested an API removed in the go-text/typesetting
// migration and were dropped; the current font internals are covered by the
// untagged font_*_test.go suites.)

func loadSystemFonts(fm *fontscan.FontMap) error {
	return nil
}

func scanSystemFontFamilies() []FontFamilyInfo {
	return nil
}
