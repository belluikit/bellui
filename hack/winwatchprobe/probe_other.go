//go:build !windows

package main

import "fmt"

func run() { fmt.Println("winwatchprobe only does anything on Windows (see belluikit/bellui#68)") }
