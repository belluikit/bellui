package metadata

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadAppMetadata(t *testing.T) {
	r, err := os.Open("./testdata/manifest.pxf")
	assert.Nil(t, err)
	defer r.Close()

	data, err := Load(r)
	assert.Nil(t, err)
	assert.Equal(t, "https://apps.bellui.io", data.Website)
	assert.Equal(t, "io.bellui.bellui", data.Details.ID)
	assert.Equal(t, "1.0.0", data.Details.Version)
	assert.Equal(t, 1, data.Details.Build)
	assert.Equal(t, data.Release["Test"], "Value1")
	assert.Equal(t, data.Release["InReleaseOnly"], "Value3")
	assert.NotContains(t, data.Release, "InDevelopmentOnly")
	assert.Equal(t, data.Development["Test"], "Value2")
	assert.Equal(t, data.Development["InDevelopmentOnly"], "Value4")
	assert.NotContains(t, data.Development, "InReleaseOnly")

	assert.NotNil(t, data.Source)
	assert.Equal(t, data.Source.Repo, "https://github.com/belluikit/bellui")
	assert.Equal(t, data.Source.Dir, "internal/metadata/testdata")
}
