//go:build !android

package main

func runProbe() string { return "error: android only" }
