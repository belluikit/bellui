package metadata

// BelluiApp describes the top level metadata for building a bellui application.
// It is serialized to and from the manifest.pxf file via the textual PXF format;
// the wire schema lives in belluiapppb/belluiapp.proto and the mapping in convert.go.
type BelluiApp struct {
	Website      string
	Description  string
	Details      AppDetails
	AdaptiveIcon *AdaptiveIcon

	Development map[string]string
	Release     map[string]string
	Source      *AppSource
	CanOpen     *CanOpen
	LinuxAndBSD *LinuxAndBSD
	Languages   []string
	Migrations  map[string]bool
}

// AppDetails describes the build information, this group may be OS or arch specific
type AppDetails struct {
	Icon     string
	Name, ID string
	Version  string
	Build    int

	// FileAssociations declares document types that this application can open.
	// The packaging metadata emitted from these entries is what asks the OS to
	// route matching files to the app (macOS CFBundleDocumentTypes, Android
	// ACTION_VIEW intent-filters, Linux desktop MimeType, Windows registry).
	FileAssociations []FileAssociation

	// URLSchemes declares custom URL schemes (without the "://") that this
	// application registers as a handler for, e.g. "myapp" for myapp://open.
	URLSchemes []string
}

// FileAssociation declares a single document type that the application can open.
// It maps to per-platform handler metadata emitted by the packaging commands.
type FileAssociation struct {
	// Extensions is the set of file extensions, without the leading dot, that
	// belong to this type (e.g. ["txt", "md"]).
	Extensions []string
	// MimeType is the canonical MIME type for this document type, e.g. "text/plain".
	MimeType string
	// Description is a human readable name for the type, e.g. "Text Document".
	Description string
	// Role is the macOS CFBundleTypeRole for the type: one of "Editor", "Viewer",
	// "Shell" or "None". Defaults to "Editor" when empty. Ignored on other platforms.
	Role string
	// Icon is an optional per-type icon file. Ignored where the platform has no
	// per-document-type icon slot.
	Icon string
}

type AdaptiveIcon struct {
	Foreground string
	Background string
	Monochrome string
}

type AppSource struct {
	Repo, Dir string
}

// LinuxAndBSD describes specific metadata for desktop files on Linux and BSD.
type LinuxAndBSD struct {
	GenericName string
	Categories  []string
	Comment     string
	Keywords    []string
	ExecParams  string
}

// CanOpen represents a selection of file types (mime etc) that this application can open.
type CanOpen struct {
	MimeTypes string
}
