package dialog

import (
	"image/color"
	"path/filepath"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

const (
	fileIconSize       = 48
	fileInlineIconSize = 24
	fileIconCellWidth  = fileIconSize * 1.5
)

var (
	_ event.Hoverable     = (*fileDialogItem)(nil)
	_ event.LongPressable = (*fileDialogItem)(nil)
)

type fileDialogItem struct {
	widget.BaseWidget
	picker *fileDialog

	name     string
	id       int // id in the parent container
	choose   func(id int)
	open     func()
	location bellui.URI
	dir      bool

	hovered   bool
	selected  bool // part of a multi-select set (#627)
	lastClick time.Time
}

func (i *fileDialogItem) CreateRenderer() core.WidgetRenderer {
	text := widget.NewLabelWithStyle(i.name, bellui.TextAlignCenter, bellui.TextStyle{})
	text.Truncation = bellui.TextTruncateEllipsis
	text.Wrapping = bellui.TextWrapBreak
	icon := widget.NewFileIcon(i.location)

	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = i.Theme().Size(theme.SizeNameShapeSmall)

	return &fileItemRenderer{
		item:         i,
		icon:         icon,
		text:         text,
		bg:           bg,
		objects:      []core.CanvasObject{bg, icon, text},
		fileTextSize: widget.NewLabel("M\nM").MinSize().Height, // cache two-line label height,
	}
}

func (i *fileDialogItem) setLocation(l bellui.URI, dir, up bool) {
	i.dir = dir
	i.location = l
	i.name = l.Name()

	if i.picker.view == GridView {
		ext := filepath.Ext(i.name[1:])
		i.name = i.name[:len(i.name)-len(ext)]
	}

	if up {
		i.name = "(" + lang.X("file.parent", "Parent") + ")"
	}

	i.Refresh()
}

func (i *fileDialogItem) Tapped(*event.PointEvent) {
	if i.choose != nil {
		i.choose(i.id)
	}
	if i.picker.selectionMode {
		// In selection mode taps toggle set membership (#259); the double-tap
		// open shortcut is suspended — its second tap just changed the set it
		// would confirm.
		i.lastClick = time.Time{}
		return
	}
	now := time.Now()
	if !i.dir && now.Sub(i.lastClick) < bellui.DoubleTapDelay() && i.open != nil {
		i.open()
	}
	i.lastClick = now
}

// LongPressed enters touch selection mode (#259): holding a file item switches
// a multi-select dialog into a mode where plain taps toggle set membership, so
// devices without modifier keys can build a set. Hold-fire dispatch (#758)
// sends no Tapped on the release of the press that got here, so the held item
// is selected exactly once. Ignored on directories and outside multi-select.
func (i *fileDialogItem) LongPressed(*event.PointEvent) {
	i.picker.enterSelectionMode(i.id)
}

func (i *fileDialogItem) HoverIn(*event.HoverEvent)    { i.hovered = true; i.Refresh() }
func (i *fileDialogItem) HoverMoved(*event.HoverEvent) {}
func (i *fileDialogItem) HoverOut()                    { i.hovered = false; i.Refresh() }

func (f *fileDialog) newFileItem(location bellui.URI, dir, up bool) *fileDialogItem {
	item := &fileDialogItem{
		picker:   f,
		location: location,
		name:     location.Name(),
		dir:      dir,
	}

	if f.view == GridView {
		ext := filepath.Ext(item.name[1:])
		item.name = item.name[:len(item.name)-len(ext)]
	}

	if up {
		item.name = "(" + lang.X("file.parent", "Parent") + ")"
	}

	item.ExtendBaseWidget(item)
	return item
}

type fileItemRenderer struct {
	item         *fileDialogItem
	fileTextSize float32

	bg      *canvas.Rectangle
	icon    *widget.FileIcon
	text    *widget.Label
	objects []core.CanvasObject
}

func (s *fileItemRenderer) Layout(size geo.Size) {
	s.bg.Resize(size)
	s.bg.Move(geo.NewPos(0, 0))

	if s.item.picker.view == GridView {
		padding := theme.Padding()
		s.icon.Resize(geo.NewSize(fileIconSize, fileIconSize))
		s.icon.Move(geo.NewPos((size.Width-fileIconSize)/2, padding))

		s.text.Alignment = bellui.TextAlignCenter
		s.text.Resize(geo.NewSize(size.Width-padding*2, s.fileTextSize))
		s.text.Move(geo.NewPos(padding, size.Height-s.fileTextSize-padding))
	} else {
		s.icon.Resize(geo.NewSize(fileInlineIconSize, fileInlineIconSize))
		s.icon.Move(geo.NewPos(theme.Padding()*2, (size.Height-fileInlineIconSize)/2))

		s.text.Alignment = bellui.TextAlignLeading
		textMin := s.text.MinSize()
		s.text.Resize(geo.NewSize(size.Width-fileInlineIconSize-theme.Padding()*3, textMin.Height))
		s.text.Move(geo.NewPos(fileInlineIconSize+theme.Padding()*3, (size.Height-textMin.Height)/2))
	}
}

func (s *fileItemRenderer) MinSize() geo.Size {
	padding := theme.Padding()
	if s.item.picker.view == GridView {
		return geo.NewSize(fileIconCellWidth, fileIconSize+s.fileTextSize+padding*2)
	}

	textMin := s.text.MinSize()
	return geo.NewSize(fileInlineIconSize+textMin.Width+padding*4, textMin.Height+padding*2)
}

func (s *fileItemRenderer) Refresh() {
	s.fileTextSize = widget.NewLabel("M\nM").MinSize().Height

	th := s.item.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	switch {
	case s.item.selected:
		// Multi-select highlight (#627). Blend the selection color with a hover
		// layer so a hovered selected item still reads as hovered.
		s.bg.FillColor = th.Color(theme.ColorNameSelection, v)
	case s.item.hovered:
		onSurface := th.Color(theme.ColorNameOnSurface, v)
		s.bg.FillColor = theme.StateLayerHover(onSurface)
	default:
		s.bg.FillColor = color.Transparent
	}
	s.bg.CornerRadius = s.item.Theme().Size(theme.SizeNameShapeSmall)
	s.bg.Refresh()

	s.text.SetText(s.item.name)
	s.icon.SetURI(s.item.location)
}

func (s *fileItemRenderer) Objects() []core.CanvasObject {
	return s.objects
}

func (s *fileItemRenderer) Destroy() {
}
