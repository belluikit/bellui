//go:build tamago || noos

package dialog

import "github.com/belluikit/bellui"

func getFavoriteLocations() (map[string]bellui.ListableURI, error) {
	return map[string]bellui.ListableURI{}, nil
}
