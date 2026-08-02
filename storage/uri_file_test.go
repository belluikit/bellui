// TestFileAbs asserts real cwd/home path resolution the browser shim does not model.
// (#77's split convention; #79 Phase B.)
//go:build !wasm

package storage_test

import (
	"os"
	"testing"

	"github.com/belluikit/bellui/storage"
	"github.com/stretchr/testify/assert"
)

func TestFileAbs(t *testing.T) {
	pwd, err := os.Getwd()
	if err != nil {
		t.Error("Could not get working directory")
		defer os.Chdir(pwd)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Error("Could not get user home directory")
	}

	os.Chdir(home)

	abs := storage.NewFileURI(home)
	rel := storage.NewFileURI(".")

	assert.Equal(t, abs.Path(), rel.Path())
	assert.Equal(t, abs.String(), rel.String())

	assert.Equal(t, "file:///", storage.NewFileURI("/").String())
}
