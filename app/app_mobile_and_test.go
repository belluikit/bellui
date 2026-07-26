//go:build !ci && android

package app

import (
	"os"
	"testing"

	"github.com/belluikit/bellui/internal/app"

	"github.com/stretchr/testify/assert"
)

func Test_RootConfigDir(t *testing.T) {
	oldEnv := os.Getenv("FILESPATH")
	os.Setenv("FILESPATH", "/tmp")

	assert.Equal(t, "/tmp", app.RootConfigDir())
	os.Setenv("FILESPATH", oldEnv)
}

func Test_StoragePath(t *testing.T) {
	oldEnv := os.Getenv("FILESPATH")
	os.Setenv("FILESPATH", "/tmp")
	defer os.Setenv("FILESPATH", oldEnv)

	p := &preferences{}
	expected := "/tmp/storage/preferences.pxf"
	assert.Equal(t, expected, p.storagePath())
}
