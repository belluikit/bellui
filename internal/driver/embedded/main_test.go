package embedded_test

import (
	"testing"

	"github.com/belluikit/bellui/test"
)

// TestMain installs the test app for the whole binary. The embedded driver
// renders through the in-memory software canvas, whose Capture resolves theme
// colours from bellui.CurrentApp — nil, and a panic, without one. These tests
// used to inherit an app from bellui/test's init, pulled in transitively by
// driver/software; that import is gone (#304), so the dependency is declared
// here instead of being an accident of the import graph.
func TestMain(m *testing.M) {
	test.NewApp()
	m.Run()
}
