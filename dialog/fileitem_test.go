package dialog

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/internal/cache"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

func TestFileItem_Name(t *testing.T) {
	f := &fileDialog{file: &FileDialog{}}
	_ = f.makeUI()

	item := f.newFileItem(storage.NewFileURI("/path/to/filename.txt"), false, false)
	assert.Equal(t, "filename", item.name)

	item = f.newFileItem(storage.NewFileURI("/path/to/MyFile.jpeg"), false, false)
	assert.Equal(t, "MyFile", item.name)

	item = f.newFileItem(storage.NewFileURI("/path/to/.maybeHidden.txt"), false, false)
	assert.Equal(t, ".maybeHidden", item.name)

	item = f.newFileItem(storage.NewFileURI("/path/to/noext"), false, false)
	assert.Equal(t, "noext", item.name)

	// Test that the extension remains for the list view.
	f.view = ListView

	item = f.newFileItem(storage.NewFileURI("/path/to/filename.txt"), false, false)
	assert.Equal(t, "filename.txt", item.name)

	item = f.newFileItem(storage.NewFileURI("/path/to/MyFile.jpeg"), false, false)
	assert.Equal(t, "MyFile.jpeg", item.name)

	item = f.newFileItem(storage.NewFileURI("/path/to/.maybeHidden.txt"), false, false)
	assert.Equal(t, ".maybeHidden.txt", item.name)

	item = f.newFileItem(storage.NewFileURI("/path/to/noext"), false, false)
	assert.Equal(t, "noext", item.name)
}

func TestFileItem_FolderName(t *testing.T) {
	f := &fileDialog{file: &FileDialog{}}
	_ = f.makeUI()

	item := f.newFileItem(storage.NewFileURI("/path/to/foldername/"), true, false)
	assert.Equal(t, "foldername", item.name)

	item = f.newFileItem(storage.NewFileURI("/path/to/myapp.app/"), true, false)
	assert.Equal(t, "myapp", item.name)

	item = f.newFileItem(storage.NewFileURI("/path/to/.maybeHidden/"), true, false)
	assert.Equal(t, ".maybeHidden", item.name)

	// Test that the extension remains for the list view.
	f.view = ListView
	item = f.newFileItem(storage.NewFileURI("/path/to/myapp.app/"), true, false)
	assert.Equal(t, "myapp.app", item.name)
}

func TestNewFileItem(t *testing.T) {
	f := &fileDialog{file: &FileDialog{}}
	_ = f.makeUI()
	item := f.newFileItem(storage.NewFileURI("/path/to/filename.txt"), false, false)

	assert.Equal(t, "filename", item.name)
}

func TestFileItem_Wrap(t *testing.T) {
	f := &fileDialog{file: &FileDialog{}}
	_ = f.makeUI()
	item := f.newFileItem(storage.NewFileURI("/path/to/filename.txt"), false, false)
	item.Resize(item.MinSize())
	label := test.TempWidgetRenderer(t, item).(*fileItemRenderer).text
	assert.Equal(t, "filename", label.Text)
	texts := test.TempWidgetRenderer(t, label).Objects()
	assert.Len(t, texts, 1)

	item.setLocation(storage.NewFileURI("/path/to/averylongfilename.svg"), false, false)
	rich := test.TempWidgetRenderer(t, label).Objects()[0].(*widget.RichText)
	texts = test.TempWidgetRenderer(t, rich).Objects()
	assert.Len(t, texts, 2)
	assert.Equal(t, "averylo", texts[0].(*canvas.Text).Text)
}

// TestFileItem_ReleasesMeasuringLabel checks that a file item leaves no
// renderer behind for the label it builds only to measure the text height,
// on creation and on every Refresh (#722).
func TestFileItem_ReleasesMeasuringLabel(t *testing.T) {
	test.NewTempApp(t)
	f := &fileDialog{file: &FileDialog{}}
	_ = f.makeUI()
	before := cache.RenderedWidgets()

	item := f.newFileItem(storage.NewFileURI("/path/to/filename.txt"), false, false)
	r := cache.Renderer(item)
	for range 3 {
		r.Refresh()
	}
	intdriver.DestroyRenderers(item)

	for w := range cache.RenderedWidgets() {
		if _, ok := before[w]; !ok {
			t.Errorf("renderer left behind for a %T", w)
		}
	}
}
