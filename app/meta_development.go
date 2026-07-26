package app

import (
	"os"
	"path/filepath"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/build"
	"github.com/belluikit/bellui/internal/metadata"
)

func checkLocalMetadata() {
	if build.NoMetadata || build.Mode == bellui.BuildRelease {
		return
	}

	dir := getProjectPath()
	file := filepath.Join(dir, metadata.ManifestFile)
	ref, err := os.Open(file)
	if err != nil { // no worries, this is just an optional fallback
		return
	}
	defer ref.Close()

	data, err := metadata.Load(ref)
	if err != nil || data == nil {
		bellui.LogError("failed to parse "+metadata.ManifestFile, err)
		return
	}

	meta.ID = data.Details.ID
	meta.Name = data.Details.Name
	meta.Version = data.Details.Version
	meta.Build = data.Details.Build

	if data.Details.Icon != "" {
		res, err := bellui.LoadResourceFromPath(data.Details.Icon)
		if err == nil {
			meta.Icon = metadata.ScaleIcon(res, 512)
		}
	}

	meta.Release = false
	meta.Custom = data.Development
	meta.Migrations = data.Migrations
}
