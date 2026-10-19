//go:build race

package common

// raceEnabled reports a -race build, where sync.Pool drops items on purpose,
// so allocation counts through a pool are not meaningful.
const raceEnabled = true
