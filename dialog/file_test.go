// The canvas file browser is exercised here against a real testdata directory —
// storage.ListerForURI, os paths and hideFile, which has no wasm definition at
// all (file_unix.go is `!wasm && !js`, file_wasm.go defines none). A browser has
// no filesystem to browse: the web target uses an <input type=file> instead
// (file_wasm.go), so this suite is desktop/mobile-only rather than something to
// stub out (#77).
//go:build !wasm

package dialog

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	intWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/lang"
	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

func TestEffectiveStartingDir(t *testing.T) {
	homeString, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("os.Gethome() failed, cannot run this test on this system (error stat()-ing ../) error was '%s'", err)
	}
	home, err := storage.ListerForURI(storage.NewFileURI(homeString))
	if err != nil {
		t.Skipf("could not get lister for working directory: %s", err)
	}

	parentURI, err := storage.Parent(home)
	if err != nil {
		t.Skipf("Could not get parent of working directory: %s", err)
	}

	parent, err := storage.ListerForURI(parentURI)
	if err != nil {
		t.Skipf("Could not get lister for parent of working directory: %s", err)
	}

	dialog := &FileDialog{}

	// test that we get wd when running with the default struct values
	res := dialog.effectiveStartingDir()
	expect := home
	if !storage.EqualURI(res, expect) {
		t.Errorf("Expected effectiveStartingDir() to be '%s', but it was '%s'",
			expect, res)
	}

	// this should always be equivalent to the preceding test
	dialog.startingLocation = nil
	res = dialog.effectiveStartingDir()
	expect = home
	if !storage.EqualURI(res, expect) {
		t.Errorf("Expected effectiveStartingDir() to be '%s', but it was '%s'",
			expect, res)
	}

	// check using StartingDirectory with some other directory
	dialog.startingLocation = parent
	res = dialog.effectiveStartingDir()
	expect = parent
	if res != expect {
		t.Errorf("Expected effectiveStartingDir() to be '%s', but it was '%s'",
			expect, res)
	}

	// make sure we fail over if the specified directory does not exist
	dialog.startingLocation, err = storage.ListerForURI(storage.NewFileURI("/some/file/that/does/not/exist"))
	if err == nil {
		t.Errorf("Should have failed to create lister for nonexistent file")
	}
	res = dialog.effectiveStartingDir()
	expect = home
	if res.String() != expect.String() {
		t.Errorf("Expected effectiveStartingDir() to be '%s', but it was '%s'",
			expect, res)
	}
}

func TestFileDialogStartRemember(t *testing.T) {
	start, err := storage.ListerForURI(storage.NewFileURI("testdata"))
	assert.NoError(t, err)

	w := test.NewTempWindow(t, widget.NewLabel("Content"))
	d := NewFileOpen(nil, w)
	d.SetLocation(start)
	d.Show()

	assert.Equal(t, start.String(), d.dialog.dir.String())
	d.Hide()

	d2 := NewFileOpen(nil, w)
	d2.Show()
	assert.Equal(t, start.String(), d.dialog.dir.String())
	d2.Hide()
}

func TestFileDialogResize(t *testing.T) {
	win := test.NewTempWindow(t, widget.NewLabel("Content"))
	win.Resize(geo.NewSize(600, 400))
	file := NewFileOpen(func(file bellui.URIReadCloser, err error) {}, win)
	file.SetFilter(storage.NewExtensionFileFilter([]string{".png"}))

	// Mimic the fileopen dialog
	d := &fileDialog{file: file}
	open := widget.NewButton("open", func() {})
	ui := container.NewBorder(nil, nil, nil, open)
	pad := theme.Padding()
	itemMin := d.newFileItem(storage.NewFileURI("filename.txt"), false, false).MinSize()
	originalSize := ui.MinSize().Add(itemMin.AddWidthHeight(itemMin.Width+pad*6, pad*3))
	d.win = widget.NewModalPopUp(ui, file.parent.Canvas())
	d.win.Resize(originalSize)
	file.dialog = d

	// Test resize - normal size scenario
	size := geo.NewSize(200, 180) // normal size to fit (600,400)
	file.Resize(size)
	expectedWidth := float32(200)
	assert.Equal(t, expectedWidth, file.dialog.win.Content.Size().Width+theme.Padding()*2)
	expectedHeight := float32(180)
	assert.Equal(t, expectedHeight, file.dialog.win.Content.Size().Height+theme.Padding()*2)
	// Test resize - normal size scenario again
	size = geo.NewSize(300, 280) // normal size to fit (600,400)
	file.Resize(size)
	expectedWidth = 300
	assert.Equal(t, expectedWidth, file.dialog.win.Content.Size().Width+theme.Padding()*2)
	expectedHeight = 280
	assert.Equal(t, expectedHeight, file.dialog.win.Content.Size().Height+theme.Padding()*2)

	// Test resize - greater than max size scenario
	size = geo.NewSize(800, 600)
	file.Resize(size)
	expectedWidth = 600                                          // since win width only 600
	assert.Equal(t, expectedWidth, file.dialog.win.Size().Width) // max, also work
	assert.Equal(t, expectedWidth, file.dialog.win.Content.Size().Width+theme.Padding()*2)
	expectedHeight = 400                                           // since win height only 400
	assert.Equal(t, expectedHeight, file.dialog.win.Size().Height) // max, also work
	assert.Equal(t, expectedHeight, file.dialog.win.Content.Size().Height+theme.Padding()*2)

	// Test again - extreme small size
	size = geo.NewSize(1, 1)
	file.Resize(size)
	expectedWidth = file.dialog.win.Content.MinSize().Width
	assert.Equal(t, expectedWidth, file.dialog.win.Content.Size().Width)
	expectedHeight = file.dialog.win.Content.MinSize().Height
	assert.Equal(t, expectedHeight, file.dialog.win.Content.Size().Height)
}

func TestShowFileOpen(t *testing.T) {
	var chosen bellui.URIReadCloser
	var openErr error
	win := test.NewTempWindow(t, widget.NewLabel("Content"))
	d := NewFileOpen(func(file bellui.URIReadCloser, err error) {
		chosen = file
		openErr = err
	}, win)
	testData := storage.NewFileURI("testdata")
	dir, err := storage.ListerForURI(testData)
	if err != nil {
		t.Error("Failed to open testdata dir", err)
	}
	d.SetLocation(dir)
	d.Show()

	popup := win.Canvas().Overlays().Top().(*widget.PopUp)
	defer win.Canvas().Overlays().Remove(popup)
	assert.NotNil(t, popup)

	// Use dialog fields directly instead of navigating the widget tree
	fd := d.dialog
	assert.Equal(t, lang.L("Open")+" "+lang.L("File"), fd.title.Text)
	assert.Equal(t, theme.ListIcon().Name(), fd.toggleViewButton.Icon.Name())

	nameLabel := fd.fileName.(*widget.Label)
	open := fd.open
	breadcrumb := fd.breadcrumb

	// Check breadcrumb chips
	assert.NotEmpty(t, breadcrumb.Objects)
	assert.NoError(t, err)
	components := strings.Split(testData.Path(), "/")
	if components[0] == "" {
		components[0] = "/"
	}
	// Extract chip texts (skip chevron separators)
	var chipTexts []string
	for _, object := range breadcrumb.Objects {
		if chip, ok := object.(*widget.Chip); ok {
			chipTexts = append(chipTexts, chip.Text)
		}
	}
	if assert.Equal(t, len(components), len(chipTexts)) {
		for i, text := range chipTexts {
			assert.Equal(t, components[i], text, fmt.Sprintf("Failure for %s at component %d", testData.Path(), i))
		}
	}

	files := fd.files.(*widget.GridWrap)
	objects := test.TempWidgetRenderer(t, files).Objects()[0].(*container.Scroll).Content.(*bellui.Container).Objects
	assert.NotEmpty(t, objects)

	fileName := test.TempWidgetRenderer(t, objects[0].(core.Widget)).Objects()[1].(*fileDialogItem).name
	assert.Equal(t, lang.L("(Parent)"), fileName)
	assert.True(t, open.Disabled())

	var target *fileDialogItem
	id := 0
	for i, icon := range objects {
		item := test.TempWidgetRenderer(t, icon.(core.Widget)).Objects()[1].(*fileDialogItem)
		if item.dir == false {
			target = item
			id = i
			break
		}
	}
	assert.NotNil(t, target, "Failed to find file in testdata")
	fd.files.(*widget.GridWrap).Select(id)
	assert.Equal(t, target.location.Name(), nameLabel.Text)
	assert.False(t, open.Disabled())

	test.Tap(open)
	assert.Nil(t, win.Canvas().Overlays().Top())
	assert.NoError(t, openErr)

	assert.Equal(t, target.location.String(), chosen.URI().String())

	err = chosen.Close()
	assert.NoError(t, err)
}

func TestHiddenFiles(t *testing.T) {
	dir, err := storage.ListerForURI(storage.NewFileURI("testdata"))
	assert.NoError(t, err)

	// git does not preserve windows hidden flag, so we have to set it.
	// just an empty function for non windows builds
	hidden, _ := storage.Child(dir, ".hidden")
	err = hideFile(hidden.Path())
	assert.NoError(t, err)

	win := test.NewTempWindow(t, widget.NewLabel("Content"))
	d := NewFileOpen(func(file bellui.URIReadCloser, err error) {
	}, win)
	d.SetLocation(dir)
	d.Show()

	popup := win.Canvas().Overlays().Top().(*widget.PopUp)
	defer win.Canvas().Overlays().Remove(popup)
	assert.NotNil(t, popup)

	fd := d.dialog
	files := fd.files.(*widget.GridWrap)
	objects := test.TempWidgetRenderer(t, files).Objects()[0].(*container.Scroll).Content.(*bellui.Container).Objects
	assert.NotEmpty(t, objects)

	var target *fileDialogItem
	for _, icon := range objects {
		item := test.TempWidgetRenderer(t, icon.(core.Widget)).Objects()[1].(*fileDialogItem)
		if item.name == ".hidden" {
			target = item
		}
	}
	assert.Nil(t, target, "Failed, .hidden found in testdata")

	fd.showHidden = true
	fd.refreshDir(fd.dir)

	for _, icon := range objects {
		item := test.TempWidgetRenderer(t, icon.(core.Widget)).Objects()[1].(*fileDialogItem)
		if item.name == ".hidden" {
			target = item
		}
	}
	assert.NotNil(t, target, "Failed, .hidden not found in testdata")
}

func TestSetShowHidden(t *testing.T) {
	dir, err := storage.ListerForURI(storage.NewFileURI("testdata"))
	assert.NoError(t, err)

	// git does not preserve windows hidden flag, so we have to set it.
	// just an empty function for non windows builds
	hidden, _ := storage.Child(dir, ".hidden")
	err = hideFile(hidden.Path())
	assert.NoError(t, err)

	win := test.NewTempWindow(t, widget.NewLabel("Content"))

	t.Run("before show", func(t *testing.T) {
		d := NewFileOpen(func(file bellui.URIReadCloser, err error) {
		}, win)
		d.SetLocation(dir)
		d.SetShowHidden(true)
		d.Show()

		popup := win.Canvas().Overlays().Top().(*widget.PopUp)
		defer win.Canvas().Overlays().Remove(popup)
		assert.NotNil(t, popup)

		fd := d.dialog
		files := fd.files.(*widget.GridWrap)
		objects := test.TempWidgetRenderer(t, files).Objects()[0].(*container.Scroll).Content.(*bellui.Container).Objects

		var target *fileDialogItem
		for _, icon := range objects {
			item := test.TempWidgetRenderer(t, icon.(core.Widget)).Objects()[1].(*fileDialogItem)
			if item.name == ".hidden" {
				target = item
			}
		}
		assert.NotNil(t, target, ".hidden should be visible when SetShowHidden(true) is called before Show()")
	})

	t.Run("after show", func(t *testing.T) {
		d := NewFileOpen(func(file bellui.URIReadCloser, err error) {
		}, win)
		d.SetLocation(dir)
		d.Show()

		popup := win.Canvas().Overlays().Top().(*widget.PopUp)
		defer win.Canvas().Overlays().Remove(popup)
		assert.NotNil(t, popup)

		fd := d.dialog
		files := fd.files.(*widget.GridWrap)
		objects := test.TempWidgetRenderer(t, files).Objects()[0].(*container.Scroll).Content.(*bellui.Container).Objects

		// hidden files should not be visible initially
		var target *fileDialogItem
		for _, icon := range objects {
			item := test.TempWidgetRenderer(t, icon.(core.Widget)).Objects()[1].(*fileDialogItem)
			if item.name == ".hidden" {
				target = item
			}
		}
		assert.Nil(t, target, ".hidden should not be visible by default")

		// call SetShowHidden on already-shown dialog
		d.SetShowHidden(true)

		target = nil
		for _, icon := range objects {
			item := test.TempWidgetRenderer(t, icon.(core.Widget)).Objects()[1].(*fileDialogItem)
			if item.name == ".hidden" {
				target = item
			}
		}
		assert.NotNil(t, target, ".hidden should be visible after SetShowHidden(true) on shown dialog")
	})
}

func TestShowFileSave(t *testing.T) {
	var chosen bellui.URIWriteCloser
	var saveErr error
	win := test.NewTempWindow(t, widget.NewLabel("Content"))
	saver := NewFileSave(func(file bellui.URIWriteCloser, err error) {
		chosen = file
		saveErr = err
	}, win)
	saver.Show()

	popup := win.Canvas().Overlays().Top().(*widget.PopUp)
	defer win.Canvas().Overlays().Remove(popup)
	assert.NotNil(t, popup)

	fd := saver.dialog
	assert.Equal(t, "Save File", fd.title.Text)

	nameEntry := fd.fileName.(*widget.Entry)
	save := fd.open

	files := fd.files.(*widget.GridWrap)
	objects := test.TempWidgetRenderer(t, files).Objects()[0].(*container.Scroll).Content.(*bellui.Container).Objects
	assert.NotEmpty(t, objects)

	item := test.TempWidgetRenderer(t, objects[0].(core.Widget)).Objects()[1].(*fileDialogItem)
	assert.Equal(t, lang.L("(Parent)"), item.name)
	assert.True(t, save.Disabled())

	dir, err := storage.ListerForURI(storage.NewFileURI("testdata"))
	assert.NoError(t, err)
	saver.SetLocation(dir)

	var target *fileDialogItem
	id := -1
	for i, icon := range objects {
		item := test.TempWidgetRenderer(t, icon.(core.Widget)).Objects()[1].(*fileDialogItem)
		if item.dir == false {
			target = item
			id = i
			break
		}
	}

	if target == nil {
		log.Println("Could not find a file in the default directory to tap :(")
		return
	}

	fd.files.(*widget.GridWrap).Select(id)
	assert.Equal(t, target.location.Name(), nameEntry.Text)
	assert.False(t, save.Disabled())

	// we are about to overwrite, a warning will show
	test.Tap(save)
	confirmUI := win.Canvas().Overlays().Top().(*widget.PopUp)
	assert.NotEqual(t, confirmUI, popup)
	confirmUI.Hide()

	// give the file a unique name and it will callback fine
	nameEntry.SetText("v2_" + nameEntry.Text)
	test.Tap(save)
	assert.Nil(t, win.Canvas().Overlays().Top())
	assert.NoError(t, saveErr)
	targetParent, err := storage.Parent(target.location)
	if err != nil {
		t.Error(err)
	}
	expectedPath, _ := storage.Child(targetParent, "v2_"+target.location.Name())
	assert.Equal(t, expectedPath.String(), chosen.URI().String())

	err = chosen.Close()
	assert.NoError(t, err)
	pathString := expectedPath.Path()
	err = os.Remove(pathString)
	assert.NoError(t, err)
}

func TestFileFilters(t *testing.T) {
	win := test.NewTempWindow(t, widget.NewLabel("Content"))
	f := NewFileOpen(func(file bellui.URIReadCloser, err error) {
	}, win)

	f.SetFilter(storage.NewExtensionFileFilter([]string{".png"}))
	f.Show()

	workingDir, err := os.Getwd()
	if err != nil {
		bellui.LogError("Could not get current working directory", err)
		t.FailNow()
	}
	testDataDir := storage.NewFileURI(filepath.Join(workingDir, "testdata"))
	testDataLister, err := storage.ListerForURI(testDataDir)
	if err != nil {
		t.Error(err)
	}

	f.dialog.setLocation(testDataLister)

	count := 0
	for _, uri := range f.dialog.data {
		ok, _ := storage.CanList(uri)
		if !ok {
			assert.Equal(t, ".png", uri.Extension())
			count++
		}
	}

	// NOTE: This count needs to be updated when more test images are added.
	assert.Equal(t, 10, count)

	f.SetFilter(storage.NewMimeTypeFileFilter([]string{"image/jpeg"}))

	count = 0
	for _, uri := range f.dialog.data {
		ok, _ := storage.CanList(uri)
		if !ok {
			assert.Equal(t, "image/jpeg", uri.MimeType())
			count++
		}
	}

	// NOTE: This count needs to be updated when more test images are added.
	assert.Equal(t, 1, count)

	f.SetFilter(storage.NewMimeTypeFileFilter([]string{"image/*"}))

	count = 0
	for _, uri := range f.dialog.data {
		ok, _ := storage.CanList(uri)
		if !ok {
			mimeType := strings.Split(uri.MimeType(), "/")[0]
			assert.Equal(t, "image", mimeType)
			count++
		}
	}

	// NOTE: This count needs to be updated when more test images are added.
	assert.Equal(t, 11, count)
}

func TestFileSort(t *testing.T) {
	dir, err := storage.ListerForURI(storage.NewFileURI("testdata"))
	assert.NoError(t, err)

	win := test.NewTempWindow(t, widget.NewLabel("Content"))
	d := NewFileOpen(func(file bellui.URIReadCloser, err error) {
	}, win)
	d.SetLocation(dir)
	d.Show()

	popup := win.Canvas().Overlays().Top().(*widget.PopUp)
	defer win.Canvas().Overlays().Remove(popup)
	assert.NotNil(t, popup)

	fd := d.dialog
	files := fd.files.(*widget.GridWrap)
	objects := test.TempWidgetRenderer(t, files).Objects()[0].(*container.Scroll).Content.(*bellui.Container).Objects
	assert.NotEmpty(t, objects)

	binPos := -1
	capitalPos := -1
	for i, icon := range objects {
		item := test.TempWidgetRenderer(t, icon.(core.Widget)).Objects()[1].(*fileDialogItem)
		switch item.name {
		case "bin":
			binPos = i
		case "Capitalised":
			capitalPos = i
		}
	}

	assert.NotEqual(t, -1, binPos, "bin file not found")
	assert.NotEqual(t, -1, capitalPos, "Capitalised.txt file not found")
	assert.Less(t, binPos, capitalPos)
}

func TestView(t *testing.T) {
	win := test.NewTempWindow(t, widget.NewLabel("Content"))

	dlg := NewFileOpen(func(reader bellui.URIReadCloser, err error) {
		assert.NoError(t, err)
		assert.Nil(t, reader)
	}, win)

	dlg.SetTitleText("File Selection")
	dlg.SetConfirmText("Yes")
	dlg.SetDismissText("Dismiss")
	dlg.Show()

	popup := win.Canvas().Overlays().Top().(*widget.PopUp)
	defer win.Canvas().Overlays().Remove(popup)
	assert.NotNil(t, popup)

	fd := dlg.dialog

	// view should be a grid
	_, isGrid := fd.files.(*widget.GridWrap)
	assert.True(t, isGrid)
	assert.Equal(t, theme.ListIcon().Name(), fd.toggleViewButton.Icon.Name())

	// toggle view
	test.Tap(fd.toggleViewButton)

	// view should be a list
	_, isList := fd.files.(*widget.List)
	assert.True(t, isList)
	assert.Equal(t, theme.GridIcon().Name(), fd.toggleViewButton.Icon.Name())

	// toggle view
	test.Tap(fd.toggleViewButton)

	// view should be a grid again
	_, isGrid = fd.files.(*widget.GridWrap)
	assert.True(t, isGrid)
	assert.Equal(t, theme.ListIcon().Name(), fd.toggleViewButton.Icon.Name())

	assert.Equal(t, "File Selection", fd.title.Text)
	assert.Equal(t, "Yes", fd.open.Text)
	assert.Equal(t, "Dismiss", fd.dismiss.Text)
}

func TestSetView(t *testing.T) {
	win := test.NewTempWindow(t, widget.NewLabel("Content"))

	bellui.SetPref(bellui.CurrentApp().Preferences(), viewLayoutKey, int(defaultView))

	dlg := NewFileOpen(func(reader bellui.URIReadCloser, err error) {
		assert.NoError(t, err)
		assert.Nil(t, reader)
	}, win)

	dlg.SetTitleText("File Selection")
	dlg.SetConfirmText("Yes")
	dlg.SetDismissText("Dismiss")

	// set view to list
	dlg.SetView(ListView)

	dlg.Show()

	popup := win.Canvas().Overlays().Top().(*widget.PopUp)
	defer win.Canvas().Overlays().Remove(popup)
	assert.NotNil(t, popup)

	fd := dlg.dialog

	// view should be a list
	_, isList := fd.files.(*widget.List)
	assert.True(t, isList)
	assert.Equal(t, theme.GridIcon(), fd.toggleViewButton.Icon)

	assert.Equal(t, "File Selection", fd.title.Text)
	assert.Equal(t, "Yes", fd.open.Text)
	assert.Equal(t, "Dismiss", fd.dismiss.Text)

	// set view to grid on already opened dialog - should be updated automatically
	dlg.SetView(GridView)

	// view should be a grid again
	_, isGrid := fd.files.(*widget.GridWrap)
	assert.True(t, isGrid)
	assert.Equal(t, theme.ListIcon(), fd.toggleViewButton.Icon)
}

func TestSetViewPreferences(t *testing.T) {
	win := test.NewTempWindow(t, widget.NewLabel("Content"))

	prefs := bellui.CurrentApp().Preferences()

	// set user-saved viewLayout to GridView
	bellui.SetPref(prefs, viewLayoutKey, int(GridView))

	dlg := NewFileOpen(func(reader bellui.URIReadCloser, err error) {
		assert.NoError(t, err)
		assert.Nil(t, reader)
	}, win)

	// set default view to be ListView
	dlg.SetView(ListView)

	dlg.Show()

	popup := win.Canvas().Overlays().Top().(*widget.PopUp)
	defer win.Canvas().Overlays().Remove(popup)
	assert.NotNil(t, popup)

	fd := dlg.dialog

	// check that preference setting overrules configured default view
	_, isGrid := fd.files.(*widget.GridWrap)
	assert.True(t, isGrid)
	assert.Equal(t, theme.ListIcon(), fd.toggleViewButton.Icon)
}

func TestViewPreferences(t *testing.T) {
	win := test.NewTempWindow(t, widget.NewLabel("Content"))

	prefs := bellui.CurrentApp().Preferences()

	// set viewLayout to an invalid value to verify that this situation is handled properly
	bellui.SetPref(prefs, viewLayoutKey, -1)

	dlg := NewFileOpen(func(reader bellui.URIReadCloser, err error) {
		assert.NoError(t, err)
		assert.Nil(t, reader)
	}, win)

	dlg.Show()

	popup := win.Canvas().Overlays().Top().(*widget.PopUp)
	defer win.Canvas().Overlays().Remove(popup)
	assert.NotNil(t, popup)

	fd := dlg.dialog

	// default viewLayout preference should be 'grid'
	view := ViewLayout(bellui.PrefOr[int](prefs, viewLayoutKey, 0))
	assert.Equal(t, GridView, view)

	// toggle view
	test.Tap(fd.toggleViewButton)

	// viewLayout preference should be 'list'
	view = ViewLayout(bellui.PrefOr[int](prefs, viewLayoutKey, 0))
	assert.Equal(t, ListView, view)

	// toggle view
	test.Tap(fd.toggleViewButton)

	// viewLayout preference should be 'grid' again
	view = ViewLayout(bellui.PrefOr[int](prefs, viewLayoutKey, 0))
	assert.Equal(t, GridView, view)
}

func TestFileFavorites(t *testing.T) {
	_ = test.NewApp()
	win := test.NewTempWindow(t, widget.NewLabel("Content"))

	dlg := NewFileOpen(func(reader bellui.URIReadCloser, err error) {
		assert.NoError(t, err)
		assert.Nil(t, reader)
	}, win)

	dlg.Show()

	popup := win.Canvas().Overlays().Top().(*widget.PopUp)
	defer win.Canvas().Overlays().Remove(popup)
	assert.NotNil(t, popup)

	fd := dlg.dialog

	fd.loadFavorites()
	favoriteLocations, _ := getFavoriteLocations()
	places := fd.getPlaces()
	assert.Len(t, fd.favorites, len(favoriteLocations)+len(places))

	favoritesList := fd.favoritesList
	assert.Len(t, fd.favorites, favoritesList.Length())

	for i := 0; i < favoritesList.Length(); i++ {
		favoritesList.Select(i)

		f := fd.favorites[i]
		loc, ok := favoriteLocations[f.locName]
		if ok {
			// favoriteItem is Home, Documents, Downloads
			assert.Equal(t, loc.String(), fd.dir.String())
		} else {
			// favoriteItem is (on windows) C:\, D:\, etc.
			assert.NotEqual(t, "Home", f.locName)
		}

		ok, err := storage.Exists(fd.dir)
		assert.NoError(t, err)
		assert.True(t, ok)
	}

	dlg.Dismiss()
}

func TestSetFileNameBeforeShow(t *testing.T) {
	win := test.NewTempWindow(t, widget.NewLabel("Content"))
	dSave := NewFileSave(func(bellui.URIWriteCloser, error) {}, win)
	dSave.SetFileName("testfile.zip")
	dSave.Show()

	assert.Equal(t, "testfile.zip", dSave.dialog.fileName.(*widget.Entry).Text)

	// Should have no effect on FileOpen dialog
	dOpen := NewFileOpen(func(f bellui.URIReadCloser, e error) {}, win)
	dOpen.SetFileName("testfile.zip")
	dOpen.Show()

	assert.NotEqual(t, "testfile.zip", dOpen.dialog.fileName.(*widget.Label).Text)
}

func TestSetFileNameAfterShow(t *testing.T) {
	win := test.NewTempWindow(t, widget.NewLabel("Content"))
	dSave := NewFileSave(func(bellui.URIWriteCloser, error) {}, win)
	dSave.Show()
	dSave.SetFileName("testfile.zip")

	assert.Equal(t, "testfile.zip", dSave.dialog.fileName.(*widget.Entry).Text)

	// Should have no effect on FileOpen dialog
	dOpen := NewFileOpen(func(f bellui.URIReadCloser, e error) {}, win)
	dOpen.Show()
	dOpen.SetFileName("testfile.zip")

	assert.NotEqual(t, "testfile.zip", dOpen.dialog.fileName.(*widget.Label).Text)
}

func TestTapParent_GoesUpOne(t *testing.T) {
	win := test.NewTempWindow(t, widget.NewLabel("Content"))
	d := NewFileOpen(func(bellui.URIReadCloser, error) {}, win)
	home, _ := os.UserHomeDir()
	homeURI, _ := storage.ListerForURI(storage.NewFileURI(home))
	parentURI, _ := storage.Parent(homeURI)

	d.SetView(GridView)
	d.SetLocation(homeURI)
	d.Show()

	items := test.WidgetRenderer(d.dialog.files)
	item := items.Objects()[0].(*intWidget.Scroll).Content.(*bellui.Container).Objects[0]
	parent := test.TempWidgetRenderer(t, item.(core.Widget)).Objects()[1].(*fileDialogItem)
	test.Tap(parent)

	assert.Equal(t, d.dialog.dir.String(), parentURI.String())
}

func TestCreateNewFolderInDir(t *testing.T) {
	win := test.NewTempWindow(t, widget.NewLabel("Content"))

	folderDialog := NewFolderOpen(func(lu bellui.ListableURI, err error) {
		assert.NoError(t, err)
	}, win)
	folderDialog.SetConfirmText("Choose")
	folderDialog.SetDismissText("Cancel")
	folderDialog.Show()

	folderDialogPopup := win.Canvas().Overlays().Top().(*widget.PopUp)
	defer win.Canvas().Overlays().Remove(folderDialogPopup)
	assert.NotNil(t, folderDialogPopup)

	// Find the new folder button by scanning action buttons in the header
	// The header layout has [bg, divider, title, actions]
	// We test by tapping the new folder action (3rd action button)
	// Instead, use a simpler approach: create a folder dialog and test the
	// new folder form dialog appears
	folderDialogUI := popUpBody(folderDialogPopup)
	_ = folderDialogUI

	// Find the new folder icon button by looking through the action bar.
	// The outer container is Stack(dialogBg, Border(top, bottom, center)).
	// Top section is VBox(header, breadcrumbBar, search).
	// Header is headerBarLayout([bg, divider, title, actions]).
	// Actions is HBox([search, toggle, newFolder, options]).
	outerBorder := folderDialogUI.Objects[1].(*bellui.Container)
	topSection := outerBorder.Objects[1].(*bellui.Container) // VBox
	headerBar := topSection.Objects[0].(*bellui.Container)   // headerBarLayout
	actionsHBox := headerBar.Objects[3].(*bellui.Container)  // HBox
	createNewFolderButton := actionsHBox.Objects[2].(*widget.IconButton)
	assert.Equal(t, theme.FolderNewIcon().Name(), createNewFolderButton.Icon.Name())

	// open folder name input dialog
	test.Tap(createNewFolderButton)

	inputPopup := win.Canvas().Overlays().Top().(*widget.PopUp)
	defer win.Canvas().Overlays().Remove(inputPopup)
	assert.NotNil(t, inputPopup)

	folderNameInputUI := popUpBody(inputPopup)

	folderNameInputTitle := folderNameInputUI.Objects[4].(*widget.Label)
	assert.Equal(t, "New Folder", folderNameInputTitle.Text)

	folderNameInputLabel := folderNameInputUI.Objects[2].(*widget.Form).Items[0].Text
	assert.Equal(t, "Name", folderNameInputLabel)

	folderNameInputEntry := folderNameInputUI.Objects[2].(*widget.Form).Items[0].Widget.(*widget.Entry)
	assert.Equal(t, "", folderNameInputEntry.Text)

	folderNameInputCancel := folderNameInputUI.Objects[3].(*bellui.Container).Objects[0].(*widget.Button)
	assert.Equal(t, "Cancel", folderNameInputCancel.Text)
	assert.Equal(t, theme.CancelIcon(), folderNameInputCancel.Icon)

	folderNameInputCreate := folderNameInputUI.Objects[3].(*bellui.Container).Objects[1].(*widget.Button)
	assert.Equal(t, theme.ConfirmIcon(), folderNameInputCreate.Icon)
}

func TestSetOnClosedBeforeShow(t *testing.T) {
	win := test.NewTempWindow(t, widget.NewLabel("Content"))
	d := NewFileSave(func(bellui.URIWriteCloser, error) {}, win)
	onClosedCalled := false
	d.SetOnClosed(func() { onClosedCalled = true })
	d.Show()
	d.Hide()
	assert.True(t, onClosedCalled)
}

func TestNativeFileDialogsDefaultsOn(t *testing.T) {
	t.Cleanup(func() { UseNativeFileDialogs(true) })

	assert.True(t, nativeFileDialogsEnabled(), "the platform picker is the default")

	UseNativeFileDialogs(false)
	assert.False(t, nativeFileDialogsEnabled(), "an app can still opt out")

	UseNativeFileDialogs(true)
	assert.True(t, nativeFileDialogsEnabled())
}

func TestNativeFileDialogsNeedAPlatformWindow(t *testing.T) {
	// A native picker is a window of the platform's own. Under a driver with no
	// platform windows there is nothing to put it over, so the canvas dialog
	// serves the request instead — which is also what keeps this very test suite
	// from opening real OS pickers.
	test.NewTempApp(t)
	w := test.NewTempWindow(t, widget.NewLabel("x"))

	d := NewFileOpen(func(bellui.URIReadCloser, error) {}, w)
	assert.False(t, nativeFileDialogsFor(d))
	assert.True(t, nativeFileDialogsEnabled(), "…even though the native backends are wanted in general")
}
