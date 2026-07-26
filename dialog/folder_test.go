package dialog

import (
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/lang"
	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

func TestShowFolderOpen(t *testing.T) {
	var chosen bellui.ListableURI
	var openErr error
	win := test.NewTempWindow(t, widget.NewLabel("OpenDir"))
	d := NewFolderOpen(func(file bellui.ListableURI, err error) {
		chosen = file
		openErr = err
	}, win)
	dir, err := storage.ListerForURI(storage.NewFileURI("testdata"))
	if err != nil {
		t.Error("Failed to open testdata dir", err)
	}
	d.SetLocation(dir)
	d.Show()

	popup := win.Canvas().Overlays().Top().(*widget.PopUp)
	defer win.Canvas().Overlays().Remove(popup)
	assert.NotNil(t, popup)

	ui := popUpBody(popup)
	border := ui.Objects[1].(*bellui.Container)         // Stack child 1 = Border(topSection, footer, body)
	topSection := border.Objects[1].(*bellui.Container) // Border top = VBox(header, breadcrumbBar, search)
	header := topSection.Objects[0].(*bellui.Container) // header = headerBarLayout(bg, divider, title, actions)
	title := header.Objects[2].(*widget.Label)
	assert.Equal(t, lang.L("Open")+" "+lang.L("Folder"), title.Text)

	footer := border.Objects[2].(*bellui.Container)        // Border bottom = footerBarLayout(bg, divider, content)
	footerContent := footer.Objects[2].(*bellui.Container) // footerContent = Border(nil, nil, nil, buttons, HScroll)
	nameLabel := footerContent.Objects[0].(*container.Scroll).Content.(*widget.Label)
	buttons := footerContent.Objects[1].(*bellui.Container)
	open := buttons.Objects[1].(*widget.Button)

	body := border.Objects[0].(*bellui.Container)      // Border center = Border(nil, nil, sidebar, nil, contentArea)
	contentArea := body.Objects[0].(*bellui.Container) // contentArea = Stack(contentBg, filesScroll)
	files := contentArea.Objects[1].(*container.Scroll).Content.(*bellui.Container).Objects[0].(*widget.GridWrap)
	assert.NotEmpty(t, d.dialog.data)

	item := test.TempWidgetRenderer(t, files).Objects()[0].(*container.Scroll).Content.(*bellui.Container).Objects[0]
	fileName := test.TempWidgetRenderer(t, item.(core.Widget)).Objects()[1].(*fileDialogItem).name
	assert.Equal(t, lang.L("(Parent)"), fileName)
	assert.False(t, open.Disabled())

	var target *bellui.URI
	id := -1
	for i, uri := range d.dialog.data {
		ok, _ := storage.CanList(uri)
		if ok {
			target = &uri
			id = i
		} else {
			t.Error("Folder dialog should not list files")
		}
	}

	assert.NotNil(t, target, "Failed to find folder in testdata")
	d.dialog.files.(*widget.GridWrap).Select(id)
	assert.Equal(t, (*target).Name(), nameLabel.Text)
	assert.False(t, open.Disabled())

	test.Tap(open)
	assert.Nil(t, win.Canvas().Overlays().Top())
	assert.NoError(t, openErr)

	assert.Equal(t, (*target).String(), chosen.String())
}
