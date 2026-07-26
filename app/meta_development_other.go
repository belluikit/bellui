//go:build !js

package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/belluikit/bellui"
)

func getProjectPath() string {
	exe, err := os.Executable()
	work, _ := os.Getwd()

	if err != nil {
		bellui.LogError("failed to lookup build executable", err)
		return work
	}

	temp := os.TempDir()
	if strings.Contains(exe, temp) || strings.Contains(exe, "go-build") { // this happens with "go run"
		return work
	}

	// we were called with an executable from "go build"
	return filepath.Dir(exe)
}
