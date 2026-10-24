//go:build race

package test_test

// raceEnabled reports a -race build, where sync.Pool drops items on purpose,
// so what a widget's pool of unused items holds is not deterministic.
const raceEnabled = true
