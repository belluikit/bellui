//go:build (linux || openbsd || freebsd || netbsd) && !android && !wasm && !js && !tamago && !noos

package dialog

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"
)

func getFavoriteLocation(homeURI bellui.URI, name string) (bellui.URI, error) {
	const cmdName = "xdg-user-dir"
	if _, err := exec.LookPath(cmdName); err != nil {
		return storage.Child(homeURI, name) // no lookup possible
	}

	lookupName := strings.ToUpper(name)
	cmd := exec.Command(cmdName, lookupName)
	loc, err := cmd.Output()
	if err != nil {
		return storage.Child(homeURI, name)
	}

	// Remove \n at the end
	loc = loc[:len(loc)-1]
	locURI := storage.NewFileURI(string(loc))

	if strings.TrimRight(locURI.String(), "/") == strings.TrimRight(homeURI.String(), "/") {
		fallback, _ := storage.Child(homeURI, name)
		return fallback, fmt.Errorf("this computer does not define a %s folder", lookupName)
	}

	return locURI, nil
}
