//go:build (linux || openbsd || freebsd || netbsd) && !android && !wasm && !js && !tamago && !noos

package dialog

import (
	"strings"
	"sync"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/internal/build"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/storage"

	"github.com/godbus/dbus/v5"
	"github.com/rymdport/portal"
	"github.com/rymdport/portal/filechooser"
)

const portalBusName = "org.freedesktop.portal.Desktop"

var portalReady = sync.OnceValue(func() bool {
	conn, err := dbus.SessionBus() // shared connection, don't close
	if err != nil {
		return false
	}

	var has bool
	err = conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, portalBusName).Store(&has)
	if err != nil {
		return false
	}

	return has
})

// nativePortalReady reports whether the native portal backend should handle
// this file dialog. Flatpak forces it on (the sandbox always provides a
// portal); every other desktop needs a portal actually running on the session
// bus, and needs the app not to have opted out. When it returns false the
// caller falls back to the canvas dialog.
func nativePortalReady(d *FileDialog) bool {
	if !nativeFileDialogsFor(d) {
		return false
	}
	if build.IsFlatpak {
		return true
	}

	return portalReady()
}

func openFile(parentWindowHandle string, options *filechooser.OpenFileOptions) (bellui.URIReadCloser, error) {
	title := lang.L("Open") + " " + lang.L("File")
	uri, err := open(parentWindowHandle, title, options)
	if err != nil || uri == nil {
		return nil, err
	}

	return storage.Reader(uri)
}

// openFiles is the multi-select counterpart of openFile: the portal returns
// every chosen URI and each is opened as a reader, first error aborting (pre-rebrand tracker #716).
func openFiles(parentWindowHandle string, options *filechooser.OpenFileOptions) ([]bellui.URIReadCloser, error) {
	title := lang.L("Open") + " " + lang.L("File")
	uris, err := filechooser.OpenFile(parentWindowHandle, title, options)
	if err != nil {
		return nil, err
	}

	readers := make([]bellui.URIReadCloser, 0, len(uris))
	for _, u := range uris {
		uri, err := storage.ParseURI(u)
		if err != nil {
			return nil, err
		}
		reader, err := storage.Reader(uri)
		if err != nil {
			return nil, err
		}
		readers = append(readers, reader)
	}
	return readers, nil
}

func openFolder(parentWindowHandle string, options *filechooser.OpenFileOptions) (bellui.ListableURI, error) {
	title := lang.L("Open") + " " + lang.L("Folder")
	uri, err := open(parentWindowHandle, title, options)
	if err != nil || uri == nil {
		return nil, err
	}

	return storage.ListerForURI(uri)
}

func open(parentWindowHandle, title string, options *filechooser.OpenFileOptions) (bellui.URI, error) {
	uris, err := filechooser.OpenFile(parentWindowHandle, title, options)
	if err != nil {
		return nil, err
	}

	if len(uris) == 0 {
		return nil, nil
	}

	return storage.ParseURI(uris[0])
}

func saveFile(parentWindowHandle string, options *filechooser.SaveFileOptions) (bellui.URIWriteCloser, error) {
	title := lang.L("Save") + " " + lang.L("File")
	uris, err := filechooser.SaveFile(parentWindowHandle, title, options)
	if err != nil {
		return nil, err
	}

	if len(uris) == 0 {
		return nil, nil
	}

	uri, err := storage.ParseURI(uris[0])
	if err != nil {
		return nil, err
	}

	return storage.Writer(uri)
}

func fileOpenOSOverride(d *FileDialog) bool {
	if !nativePortalReady(d) {
		return false
	}

	options := &filechooser.OpenFileOptions{
		Directory:   d.isDirectory(),
		Multiple:    d.multiSelect && !d.isDirectory(), // multi-select is file-open only (pre-rebrand tracker #716)
		AcceptLabel: d.confirmText,
	}
	if d.startingLocation != nil {
		options.CurrentFolder = d.startingLocation.Path()
	}
	options.Filters, options.CurrentFilter = convertFilterForPortal(d.filter)

	windowHandle := windowHandleForPortal(d.parent)

	// The portal helpers return (nil, nil) on cancel and (nil, err) on failure,
	// so a non-nil result is the only confirm signal notifyClosed can report.
	go func() {
		switch {
		case options.Directory:
			folder, err := openFolder(windowHandle, options)
			bellui.Do(func() {
				d.notifyClosed(folder != nil)
				folderCallback := d.callback.(func(bellui.ListableURI, error))
				folderCallback(folder, err)
			})
		case options.Multiple:
			readers, err := openFiles(windowHandle, options)
			bellui.Do(func() {
				d.notifyClosed(len(readers) > 0)
				multiCallback := d.callback.(func([]bellui.URIReadCloser, error))
				multiCallback(readers, err)
			})
		default:
			file, err := openFile(windowHandle, options)
			bellui.Do(func() {
				d.notifyClosed(file != nil)
				fileCallback := d.callback.(func(bellui.URIReadCloser, error))
				fileCallback(file, err)
			})
		}
	}()

	return true
}

func fileSaveOSOverride(d *FileDialog) bool {
	if !nativePortalReady(d) {
		return false
	}

	options := &filechooser.SaveFileOptions{
		AcceptLabel: d.confirmText,
		CurrentName: d.initialFileName,
	}
	if d.startingLocation != nil {
		options.CurrentFolder = d.startingLocation.Path()
	}
	options.Filters, options.CurrentFilter = convertFilterForPortal(d.filter)

	callback := d.callback.(func(bellui.URIWriteCloser, error))
	windowHandle := windowHandleForPortal(d.parent)

	go func() {
		file, err := saveFile(windowHandle, options)
		bellui.Do(func() {
			callback(file, err)
		})
	}()

	return true
}

func windowHandleForPortal(window bellui.Window) string {
	native, ok := window.(driver.NativeWindow)
	if !ok {
		return ""
	}

	windowHandle := ""
	if build.IsWayland {
		// Wayland: an xdg-foreign export handle of the window's toplevel, in the
		// portal's "wayland:<handle>" form. Empty when the compositor lacks
		// xdg-foreign, leaving the dialog unparented.
		native.RunNative(func(context any) {
			if handle := context.(driver.WaylandWindowContext).ExportedHandle; handle != "" {
				windowHandle = "wayland:" + handle
			}
		})
		return windowHandle
	}

	native.RunNative(func(context any) {
		handle := context.(driver.X11WindowContext).WindowHandle
		windowHandle = portal.FormatX11WindowHandle(handle)
	})
	return windowHandle
}

func convertFilterForPortal(belluiFilter storage.FileFilter) (list []*filechooser.Filter, current *filechooser.Filter) {
	if belluiFilter == nil || belluiFilter == folderFilter {
		return nil, nil
	}

	if filter, ok := belluiFilter.(*storage.ExtensionFileFilter); ok {
		rules := make([]filechooser.Rule, 0, 2*len(filter.Extensions))
		for _, ext := range filter.Extensions {
			lowercase := filechooser.Rule{
				Type:    filechooser.GlobPattern,
				Pattern: "*" + strings.ToLower(ext),
			}
			uppercase := filechooser.Rule{
				Type:    filechooser.GlobPattern,
				Pattern: "*" + strings.ToUpper(ext),
			}
			rules = append(rules, lowercase, uppercase)
		}

		name := formatFilterName(filter.Extensions, 3)
		converted := &filechooser.Filter{Name: name, Rules: rules}
		return []*filechooser.Filter{converted}, converted
	}

	if filter, ok := belluiFilter.(*storage.MimeTypeFileFilter); ok {
		rules := make([]filechooser.Rule, len(filter.MimeTypes))
		for i, mime := range filter.MimeTypes {
			rules[i] = filechooser.Rule{
				Type:    filechooser.MIMEType,
				Pattern: mime,
			}
		}

		name := formatFilterName(filter.MimeTypes, 3)
		converted := &filechooser.Filter{Name: name, Rules: rules}
		return []*filechooser.Filter{converted}, converted
	}

	return nil, nil
}

func formatFilterName(patterns []string, count int) string {
	if len(patterns) < count {
		count = len(patterns)
	}

	name := strings.Join(patterns[:count], ", ")
	if len(patterns) > count {
		name += "…"
	}

	return name
}
