//go:build mobile && !tamago && !noos

package test

// onMobileTestBuild reports whether the test binary was built with the mobile
// build tag. Golden masters then resolve to their mobile/ variants. This is a
// build-tag check rather than bellui.CurrentDevice() so it is safe in low-level
// packages whose tests run without a Bellui app/driver initialised.
func onMobileTestBuild() bool { return true }
