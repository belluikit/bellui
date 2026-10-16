package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/belluikit/bellui/internal/metadata"
)

// TestManifestIcon pins the settings app's icon to a file next to the
// manifest. `bellui package` refuses a named icon it cannot find, and silently
// substitutes the toolkit's default icon when Icon.png is missing; nothing
// else in CI reads this manifest. It named ../../theme/icons/bellui.png, which
// does not exist, until #334.
func TestManifestIcon(t *testing.T) {
	data, err := metadata.LoadStandard(".")
	if err != nil {
		t.Fatalf("load manifest.pxf: %v", err)
	}
	icon := data.Details.Icon
	if icon == "" {
		icon = "Icon.png" // what bellui package looks for when none is named
	}
	info, err := os.Stat(filepath.FromSlash(icon))
	if err != nil {
		t.Fatalf("app icon %q: %v", icon, err)
	}
	if info.IsDir() {
		t.Fatalf("app icon %q is a directory", icon)
	}
}
