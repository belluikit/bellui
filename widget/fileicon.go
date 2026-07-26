package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/repository/mime"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/theme"
)

const (
	ratioDown     = 0.45
	ratioTextSize = 0.22
)

// FileIcon is an adaption of widget.Icon for showing files and folders
//
// Since: 1.4
type FileIcon struct {
	BaseWidget

	URI bellui.URI

	resource  bellui.Resource
	extension string
}

// NewFileIcon takes a filepath and creates an icon with an overlaid label using the detected mimetype and extension
//
// Since: 1.4
func NewFileIcon(uri bellui.URI) *FileIcon {
	i := &FileIcon{URI: uri}
	i.ExtendBaseWidget(i)
	return i
}

// SetURI changes the URI and makes the icon reflect a different file
func (i *FileIcon) SetURI(uri bellui.URI) {
	i.URI = uri
	i.Refresh()
}

// must be called with i.propertyLock RLocked
func (i *FileIcon) setURI(uri bellui.URI) {
	if uri == nil {
		i.resource = i.Theme().Icon(theme.IconNameFile)
		return
	}

	i.URI = uri
	i.resource = i.lookupIcon(i.URI)
	i.extension = trimmedExtension(uri)
}

// MinSize returns the size that this widget should not shrink below
func (i *FileIcon) MinSize() geo.Size {
	i.ExtendBaseWidget(i)
	return i.BaseWidget.MinSize()
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer
func (i *FileIcon) CreateRenderer() core.WidgetRenderer {
	th := i.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	i.ExtendBaseWidget(i)
	i.setURI(i.URI)

	s := &fileIconRenderer{file: i}
	s.img = canvas.NewImageFromResource(s.file.resource)
	s.img.FillMode = canvas.ImageFillContain
	s.ext = canvas.NewText(s.file.extension, th.Color(theme.ColorNameOnSurface, v))
	s.ext.Alignment = bellui.TextAlignCenter

	s.SetObjects([]core.CanvasObject{s.img, s.ext})

	return s
}

// must be called with i.propertyLock RLocked
func (i *FileIcon) lookupIcon(uri bellui.URI) bellui.Resource {
	if icon, ok := uri.(bellui.URIWithIcon); ok {
		return icon.Icon()
	}
	if i.isDir(uri) {
		return theme.FolderIcon()
	}

	th := i.Theme()
	mainMimeType, _ := mime.Split(uri.MimeType())
	switch mainMimeType {
	case "application":
		return th.Icon(theme.IconNameFileApplication)
	case "audio":
		return th.Icon(theme.IconNameFileAudio)
	case "image":
		return th.Icon(theme.IconNameFileImage)
	case "text":
		return th.Icon(theme.IconNameFileText)
	case "video":
		return th.Icon(theme.IconNameFileVideo)
	default:
		return th.Icon(theme.IconNameFile)
	}
}

func (i *FileIcon) isDir(uri bellui.URI) bool {
	if _, ok := uri.(bellui.ListableURI); ok {
		return true
	}

	listable, err := storage.ListerForURI(uri)
	if err != nil {
		return false
	}

	i.URI = listable // Avoid having to call storage.ListerForURI(uri) the next time.
	return true
}

type fileIconRenderer struct {
	widget.BaseRenderer

	file *FileIcon

	ext *canvas.Text
	img *canvas.Image
}

func (s *fileIconRenderer) MinSize() geo.Size {
	th := s.file.Theme()
	return geo.NewSquareSize(th.Size(theme.SizeNameInlineIcon))
}

func (s *fileIconRenderer) Layout(size geo.Size) {
	isize := geo.Min(size.Width, size.Height)

	xoff := float32(0)
	yoff := (size.Height - isize) / 2

	if size.Width > size.Height {
		xoff = (size.Width - isize) / 2
	}
	yoff += isize * ratioDown

	oldSize := s.ext.TextSize
	s.ext.TextSize = float32(int(isize * ratioTextSize))
	s.ext.Resize(geo.NewSize(isize, s.ext.MinSize().Height))
	s.ext.Move(geo.NewPos(xoff, yoff))
	if oldSize != s.ext.TextSize {
		s.ext.Refresh()
	}

	s.img.Resize(size)
}

func (s *fileIconRenderer) Refresh() {
	th := s.file.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	s.file.setURI(s.file.URI)

	s.ext.Color = th.Color(theme.ColorNameOnSurface, v)

	if s.img.Resource != s.file.resource {
		s.img.Resource = s.file.resource
		s.img.Refresh()
	}
	if s.ext.Text != s.file.extension {
		s.ext.Text = s.file.extension
		s.ext.Refresh()
	}

	canvas.Refresh(s.file.super())
}

func trimmedExtension(uri bellui.URI) string {
	ext := uri.Extension()
	if len(ext) > 5 {
		ext = ext[:5]
	}
	return ext
}
