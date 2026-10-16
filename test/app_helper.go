//go:build !tamago && !noos

package test

import (
	"testing"

	"github.com/belluikit/bellui"
)

// NewTempApp returns a new dummy app and tears it down at the end of the test.
//
// Since: 1.0
func NewTempApp(t testing.TB) bellui.App {
	app := NewApp()
	t.Cleanup(func() { NewApp() })
	return app
}
