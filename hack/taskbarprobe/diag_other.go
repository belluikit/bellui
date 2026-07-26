//go:build !windows

package main

import "github.com/belluikit/bellui"

// diagWindow is a no-op off Windows.
func diagWindow(w bellui.Window) {}
