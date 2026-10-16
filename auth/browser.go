package auth

import (
	"os/exec"
	"runtime"
)

// openBrowser opens the given URL in the user's default browser. It is a
// package variable so tests can capture the URL instead.
var openBrowser = openDefaultBrowser

func openDefaultBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
