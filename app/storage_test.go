package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStore_RootURI(t *testing.T) {
	id := "io.bellui.test"
	a := &belluiApp{uniqueID: id}
	// The app constructor normally does this; a bare literal skips it. On
	// desktop-ci it is the no-op (the test driver registers file://), but on
	// wasm it registers the idbfile scheme — which makes this test the
	// genuine in-browser IndexedDB counterpart (#79): Create/Close/Remove
	// below run against a real browser's IndexedDB under wasmbrowsertest.
	a.registerRepositories()
	d := makeStoreDocs(id, &store{a: a})

	w, err := d.Create("test")
	assert.NoError(t, err)
	err = w.Close()
	assert.NoError(t, err)
	err = d.Remove("test")
	assert.NoError(t, err)
}
