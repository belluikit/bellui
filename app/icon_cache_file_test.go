package app

import (
	_ "embed"
	"os"
	"sync"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/bellui.png
var iconData []byte

func TestCachedIcon_PATH(t *testing.T) {
	// cachedIconPath writes the icon to the cache under a package-level
	// sync.Once, so it only happens once per process. Reset it here so the icon
	// is re-created on every run; otherwise the deferred cleanup below removes
	// the file and a subsequent run (e.g. under -count>1) finds it missing.
	once = sync.Once{}

	SetMetadata(bellui.AppMetadata{})
	a := &belluiApp{uniqueID: "icontest"}
	assert.Equal(t, "", a.cachedIconPath())

	a.SetIcon(bellui.NewStaticResource("dummy", iconData))
	path := a.cachedIconPath()
	if path == "" {
		t.Error("cache path not constructed")
		return
	}
	defer os.Remove(path)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, "icon.png", info.Name())
}
