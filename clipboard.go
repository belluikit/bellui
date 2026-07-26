package bellui

import "image"

// Clipboard represents the system clipboard interface
type Clipboard interface {
	// Content returns the clipboard content
	Content() string
	// SetContent sets the clipboard content
	SetContent(content string)
}

// ImageClipboard is an optional interface a [Clipboard] can implement to carry
// raster images. Assert it on the value returned by [App.Clipboard]; on
// platforms without image clipboard support the assertion fails.
//
// Since: 2.16
type ImageClipboard interface {
	// Image returns the image on the clipboard, or (nil, false) when the
	// clipboard holds no image or reading it failed.
	Image() (image.Image, bool)
	// SetImage replaces the clipboard content with img.
	SetImage(img image.Image)
}

// FileClipboard is an optional interface a [Clipboard] can implement to carry
// lists of files, interoperating with copy/paste in the platform file
// manager. Assert it on the value returned by [App.Clipboard]; on platforms
// without file clipboard support the assertion fails.
//
// Since: 2.16
type FileClipboard interface {
	// Files returns the list of files on the clipboard, or nil when the
	// clipboard holds no files.
	Files() []URI
	// SetFiles replaces the clipboard content with the given files.
	// Only file-scheme URIs can be placed on the system clipboard.
	SetFiles(files []URI)
}

// HTMLClipboard is an optional interface a [Clipboard] can implement to carry
// an HTML flavor alongside plain text. Assert it on the value returned by
// [App.Clipboard]; on platforms without rich clipboard support the assertion
// fails.
//
// Since: 2.16
type HTMLClipboard interface {
	// HTML returns the HTML markup on the clipboard, or ("", false) when the
	// clipboard holds no HTML flavor.
	HTML() (string, bool)
	// SetHTML replaces the clipboard content with html, writing plainFallback
	// as the plain-text flavor of the same transaction so applications that
	// cannot paste rich text receive the fallback. Content() reports
	// plainFallback afterwards.
	SetHTML(html, plainFallback string)
}

// ClipboardContent describes a multi-flavor clipboard payload: alternative
// representations of one logical content, written together in a single
// clipboard transaction by [MultiFlavorClipboard.SetFlavors]. Zero-valued
// fields are omitted from the transaction; a wholly zero value clears the
// clipboard.
//
// Since: 3.0
type ClipboardContent struct {
	// Text is the plain-text flavor, doubling as the fallback for
	// applications that cannot paste any of the richer flavors.
	// Clipboard.Content reports it after the write.
	Text string
	// HTML is the HTML markup flavor.
	HTML string
	// Image is the raster image flavor.
	Image image.Image
	// Files is the file-list flavor. Only file-scheme URIs can be placed on
	// the system clipboard; others are skipped with a logged error.
	Files []URI
}

// MultiFlavorClipboard is an optional interface a [Clipboard] can implement
// to write several flavors in one clipboard transaction — e.g. a rendered
// chart as an image together with an HTML data table and a plain-text
// fallback. The single-flavor setters ([ImageClipboard.SetImage] etc.) each
// replace the whole clipboard, so this is the only way to offer combined
// flavors. Assert it on the value returned by [App.Clipboard]; on platforms
// without multi-flavor support the assertion fails.
//
// Since: 3.0
type MultiFlavorClipboard interface {
	// SetFlavors replaces the clipboard content with every set flavor of
	// content, written as one transaction. A flavor the platform cannot
	// carry (e.g. files on the web target, which never implements
	// [FileClipboard]) is skipped with a logged error.
	SetFlavors(content ClipboardContent)
}

// ClipboardImage returns the image on the current app's clipboard.
// It returns (nil, false) when no app is running, the clipboard does not
// support images ([ImageClipboard]) or no image is present.
//
// Since: 2.16
func ClipboardImage() (image.Image, bool) {
	if a := CurrentApp(); a != nil {
		if c, ok := a.Clipboard().(ImageClipboard); ok {
			return c.Image()
		}
	}
	return nil, false
}

// SetClipboardImage places img on the current app's clipboard.
// It reports whether the clipboard supports images ([ImageClipboard]) —
// when false nothing was written.
//
// Since: 2.16
func SetClipboardImage(img image.Image) bool {
	if a := CurrentApp(); a != nil {
		if c, ok := a.Clipboard().(ImageClipboard); ok {
			c.SetImage(img)
			return true
		}
	}
	return false
}

// ClipboardFiles returns the list of files on the current app's clipboard.
// It returns nil when no app is running, the clipboard does not support file
// lists ([FileClipboard]) or no files are present.
//
// Since: 2.16
func ClipboardFiles() []URI {
	if a := CurrentApp(); a != nil {
		if c, ok := a.Clipboard().(FileClipboard); ok {
			return c.Files()
		}
	}
	return nil
}

// SetClipboardFiles places the given files on the current app's clipboard.
// It reports whether the clipboard supports file lists ([FileClipboard]) —
// when false nothing was written.
//
// Since: 2.16
func SetClipboardFiles(files []URI) bool {
	if a := CurrentApp(); a != nil {
		if c, ok := a.Clipboard().(FileClipboard); ok {
			c.SetFiles(files)
			return true
		}
	}
	return false
}

// ClipboardHTML returns the HTML markup on the current app's clipboard.
// It returns ("", false) when no app is running, the clipboard does not
// support HTML ([HTMLClipboard]) or no HTML flavor is present.
//
// Since: 2.16
func ClipboardHTML() (string, bool) {
	if a := CurrentApp(); a != nil {
		if c, ok := a.Clipboard().(HTMLClipboard); ok {
			return c.HTML()
		}
	}
	return "", false
}

// SetClipboardFlavors places every set flavor of content on the current
// app's clipboard as one transaction. It reports whether the clipboard
// supports multi-flavor writes ([MultiFlavorClipboard]) — when false nothing
// was written.
//
// Since: 3.0
func SetClipboardFlavors(content ClipboardContent) bool {
	if a := CurrentApp(); a != nil {
		if c, ok := a.Clipboard().(MultiFlavorClipboard); ok {
			c.SetFlavors(content)
			return true
		}
	}
	return false
}

// SetClipboardHTML places html on the current app's clipboard with
// plainFallback as the plain-text flavor of the same transaction.
// It reports whether the clipboard supports HTML ([HTMLClipboard]) —
// when false nothing was written.
//
// Since: 2.16
func SetClipboardHTML(html, plainFallback string) bool {
	if a := CurrentApp(); a != nil {
		if c, ok := a.Clipboard().(HTMLClipboard); ok {
			c.SetHTML(html, plainFallback)
			return true
		}
	}
	return false
}
