//go:build (linux || openbsd || freebsd || netbsd) && !android && !wasm && !js && !tamago && !noos

package dialog

import (
	"testing"

	"github.com/rymdport/portal/filechooser"
	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/storage"
)

func TestFormatFilterName(t *testing.T) {
	actual := formatFilterName([]string{"1", "2", "3"}, 4)
	assert.Equal(t, "1, 2, 3", actual)

	actual = formatFilterName([]string{"1", "2", "3"}, 3)
	assert.Equal(t, "1, 2, 3", actual)

	actual = formatFilterName([]string{"1", "2", "3"}, 2)
	assert.Equal(t, "1, 2…", actual)
}

func TestConvertFilterForPortal_Nil(t *testing.T) {
	list, current := convertFilterForPortal(nil)
	assert.Nil(t, list)
	assert.Nil(t, current)

	// The folder sentinel must not produce a filter.
	list, current = convertFilterForPortal(folderFilter)
	assert.Nil(t, list)
	assert.Nil(t, current)
}

func TestConvertFilterForPortal_Extensions(t *testing.T) {
	filter := storage.NewExtensionFileFilter([]string{".png", ".jpg"})
	list, current := convertFilterForPortal(filter)

	assert.Len(t, list, 1)
	assert.Same(t, list[0], current)
	assert.Equal(t, ".png, .jpg", current.Name)

	// Each extension expands to a lower- and upper-case glob rule.
	assert.Len(t, current.Rules, 4)
	assert.Equal(t, filechooser.GlobPattern, current.Rules[0].Type)
	assert.Equal(t, "*.png", current.Rules[0].Pattern)
	assert.Equal(t, "*.PNG", current.Rules[1].Pattern)
	assert.Equal(t, "*.jpg", current.Rules[2].Pattern)
	assert.Equal(t, "*.JPG", current.Rules[3].Pattern)
}

func TestConvertFilterForPortal_MimeTypes(t *testing.T) {
	filter := storage.NewMimeTypeFileFilter([]string{"image/png", "image/jpeg"})
	list, current := convertFilterForPortal(filter)

	assert.Len(t, list, 1)
	assert.Same(t, list[0], current)
	assert.Len(t, current.Rules, 2)
	assert.Equal(t, filechooser.MIMEType, current.Rules[0].Type)
	assert.Equal(t, "image/png", current.Rules[0].Pattern)
	assert.Equal(t, "image/jpeg", current.Rules[1].Pattern)
}

func TestNativeFileDialogsToggle(t *testing.T) {
	// Off by default (outside Flatpak).
	assert.False(t, nativeFileDialogsEnabled())

	UseNativeFileDialogs(true)
	assert.True(t, nativeFileDialogsEnabled())

	UseNativeFileDialogs(false)
	assert.False(t, nativeFileDialogsEnabled())
}
