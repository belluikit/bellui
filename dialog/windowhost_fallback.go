//go:build ios || android || mobile || wasm || js || tamago || noos

package dialog

// windowHostSupported reports whether this build target can host dialogs in
// their own native windows. Single-window platforms (mobile, web, bare metal)
// fall back to the overlay popup presentation.
const windowHostSupported = false
