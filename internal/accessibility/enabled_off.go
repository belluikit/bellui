//go:build !accessibility

package accessibility

// Enabled reports whether the toolkit was built with accessibility support
// (the `accessibility` build tag). Driver code can branch on it instead of
// duplicating build constraints; untagged builds see false and the dead
// branches are eliminated at compile time.
const Enabled = false
