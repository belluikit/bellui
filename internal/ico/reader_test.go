package ico_test

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/belluikit/bellui/internal/ico"
	"github.com/stretchr/testify/assert"
)

func TestDecodeAll(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	files, _ := filepath.Glob("testdata/*.ico")
	for _, f := range files {
		t.Log("testing:", f)
		rd, err := os.Open(f)
		assert.NoError(err, f)
		images, err := ico.DecodeAll(rd)
		rd.Close()

		// Specific Testfile with no images should error
		if f == "testdata/DFONT.ico" {
			assert.Error(err, ico.ErrorNoEntries)
			continue
		}
		assert.NoError(err, f)
		if err != nil {
			continue
		}

		for i := range images {
			var dst string
			if len(images) == 1 {
				dst = f + ".png"
			} else {
				dst = f + fmt.Sprintf("-%d.png", i)
			}
			rd, err := os.Open(dst)
			assert.NoError(err, dst)
			dstImage, err := png.Decode(rd)
			assert.NoError(err, dst)
			rd.Close()
			if err != nil {
				continue
			}
			assert.Equal(images[i].Bounds(), dstImage.Bounds())
		}
	}
}

func TestDecodeHighestRes(t *testing.T) {
	expectedWidth := 32
	rd, err := os.Open("testdata/multi.ico")
	assert.NoError(t, err)
	defer rd.Close()
	image, err := ico.Decode(rd)
	assert.NoError(t, err)
	assert.Equal(t, expectedWidth, image.Bounds().Dx())
}
