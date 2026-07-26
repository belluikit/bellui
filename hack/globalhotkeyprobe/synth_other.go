//go:build !windows

package main

import "log"

// synthHotKey is only implemented on Windows; elsewhere -auto has nothing to
// drive, so press the shortcut yourself.
func synthHotKey() {
	log.Println("(-auto key synthesis is Windows-only; press Ctrl+Alt+K yourself)")
}
