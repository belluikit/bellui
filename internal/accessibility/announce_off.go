//go:build !accessibility

package accessibility

// Announce is a no-op without the accessibility build tag; see the tagged
// implementation for the contract.
func Announce(text string, assertive bool) {}

// AddAnnouncementListener is a no-op without the accessibility build tag; see
// the tagged implementation for the contract.
func AddAnnouncementListener(fn func(text string, assertive bool)) (remove func()) {
	return func() {}
}
