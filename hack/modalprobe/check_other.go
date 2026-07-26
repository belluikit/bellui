//go:build !windows

package main

import "github.com/belluikit/bellui"

// runWindowsCheck is a no-op off Windows; the -checkwin self-check exercises the
// EnableWindow parent block, which only exists on Windows.
func runWindowsCheck(_ bellui.App, _ bellui.Window) {}
