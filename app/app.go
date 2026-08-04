// Package app provides app implementations for working with Bellui graphical interfaces.
// The fastest way to get started is to call app.New() which will normally load a new desktop application.
// If the "ci" tag is passed to go (go run -tags ci myapp.go) it will run an in-memory application.
package app // import "github.com/belluikit/bellui/app"

import (
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal"
	"github.com/belluikit/bellui/internal/app"
	intRepo "github.com/belluikit/bellui/internal/repository"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/storage/repository"
)

// Declare conformity with App interface
var _ bellui.App = (*belluiApp)(nil)

type belluiApp struct {
	driver    bellui.Driver
	clipboard bellui.Clipboard
	icon      bellui.Resource
	uniqueID  string

	cloud     bellui.CloudProvider
	lifecycle app.Lifecycle
	settings  *settings
	storage   bellui.Storage
	prefs     bellui.Preferences

	// single-instance mode (see single_instance.go)
	siEnabled bool
	siPrimary bool
	siCloser  func()
	siOnOpen  func([]string)

	// open-with events (see openevent.go)
	openMu       sync.Mutex
	onFileOpened func(bellui.URI)
	onURLOpened  func(*url.URL)
	pendingOpens []openPayload
	running      bool

	// rich notification callbacks (see notification.go)
	notifyMu       sync.Mutex
	notifyHandlers map[uint64]*notifyCallbacks
	notifyCounter  uint64
}

func (a *belluiApp) CloudProvider() bellui.CloudProvider {
	return a.cloud
}

func (a *belluiApp) Icon() bellui.Resource {
	if a.icon != nil {
		return a.icon
	}

	if a.Metadata().Icon == nil || len(a.Metadata().Icon.Content()) == 0 {
		return nil
	}
	return a.Metadata().Icon
}

func (a *belluiApp) SetIcon(icon bellui.Resource) {
	a.icon = icon
}

func (a *belluiApp) UniqueID() string {
	if a.uniqueID != "" {
		return a.uniqueID
	}
	if a.Metadata().ID != "" {
		return a.Metadata().ID
	}

	bellui.LogError("Preferences API requires a unique ID, use app.NewWithID() or the manifest.pxf ID field", nil)
	a.uniqueID = "missing-id-" + strconv.FormatInt(time.Now().Unix(), 10) // This is a fake unique - it just has to not be reused...
	return a.uniqueID
}

func (a *belluiApp) NewWindow(title string) bellui.Window {
	return a.driver.CreateWindow(title)
}

func (a *belluiApp) Run() {
	go a.lifecycle.RunEventQueue(a.driver.DoFromGoroutine)

	if !a.driver.Device().IsMobile() {
		a.settings.watchSettings()
	}

	// Capture the launch "open with" payload (argv / URL) then mark the app
	// running so buffered payloads are delivered once the loop is pumping.
	platformLaunchOpen(a)
	a.startOpenDelivery()

	a.driver.Run()
}

func (a *belluiApp) Quit() {
	for _, window := range a.driver.AllWindows() {
		window.Close()
	}

	if a.siCloser != nil {
		a.siCloser()
		a.siCloser = nil
	}

	a.driver.Quit()
	a.settings.stopWatching()
}

func (a *belluiApp) Driver() bellui.Driver {
	return a.driver
}

// Settings returns the application settings currently configured.
func (a *belluiApp) Settings() bellui.Settings {
	return a.settings
}

func (a *belluiApp) Storage() bellui.Storage {
	return a.storage
}

func (a *belluiApp) Preferences() bellui.Preferences {
	if a.UniqueID() == "" {
		bellui.LogError("Preferences API requires a unique ID, use app.NewWithID() or the manifest.pxf ID field", nil)
	}
	return a.prefs
}

func (a *belluiApp) Lifecycle() bellui.Lifecycle {
	return &a.lifecycle
}

func (a *belluiApp) newDefaultPreferences() *preferences {
	p := newPreferences(a)
	if a.uniqueID != "" {
		p.load()
	}
	return p
}

func (a *belluiApp) Clipboard() bellui.Clipboard {
	return a.clipboard
}

// New returns a new application instance with the default driver and no unique ID (unless specified in manifest.pxf)
func New() bellui.App {
	if meta.ID == "" {
		checkLocalMetadata() // if no ID passed, check if it was in toml
		if meta.ID == "" {
			internal.LogHint("Applications should be created with a unique ID using app.NewWithID()")
		}
	}
	return NewWithID(meta.ID)
}

func makeStoreDocs(id string, s *store) *internal.Docs {
	if id == "" {
		return &internal.Docs{} // an empty impl to avoid crashes
	}
	if root := s.a.storageRoot(); root != "" {
		uri, err := storage.ParseURI(root)
		if err != nil {
			uri = storage.NewFileURI(root)
		}

		exists, err := storage.Exists(uri)
		if !exists || err != nil {
			err = storage.CreateListable(uri)
			if err != nil {
				bellui.LogError("Failed to create app storage space", err)
			}
		}

		root, _ := s.docRootURI()
		return &internal.Docs{RootDocURI: root}
	} else {
		return &internal.Docs{} // an empty impl to avoid crashes
	}
}

func newAppWithDriver(d bellui.Driver, clipboard bellui.Clipboard, id string) bellui.App {
	// Android resolves its locale here, not at lang's package init — the init
	// lookup runs before the JVM is attached and settles on English (#282).
	// App creation precedes any widget construction, so UI built in main sees
	// the real locale. A no-op on every other platform.
	resolveStartupLocale()

	newApp := &belluiApp{uniqueID: id, clipboard: clipboard, driver: d}
	bellui.SetCurrentApp(newApp)

	newApp.prefs = newApp.newDefaultPreferences()
	newApp.lifecycle.InitEventQueue()
	newApp.lifecycle.SetOnStoppedHookExecuted(func() {
		if prefs, ok := newApp.prefs.(*preferences); ok {
			prefs.forceImmediateSave()
		}
	})

	newApp.registerRepositories() // for web this may provide docs / settings
	newApp.settings = loadSettings()

	// Re-apply settings on a runtime layout-direction or locale change so every
	// window is re-laid-out and repainted, reusing the same fan-out as a theme
	// change.
	bellui.RegisterLayoutDirectionRefresh(func() { newApp.settings.apply() })
	lang.RegisterLocaleChangeRefresh(func() { newApp.settings.apply() })
	store := &store{a: newApp}
	store.Docs = makeStoreDocs(id, store)
	newApp.storage = store

	httpHandler := intRepo.NewHTTPRepository()
	repository.Register("http", httpHandler)
	repository.Register("https", httpHandler)
	return newApp
}

// marker interface to pass system tray to supporting drivers
type systrayDriver interface {
	SetSystemTrayMenu(*bellui.Menu)
	SetSystemTrayIcon(bellui.Resource)
	SetSystemTrayWindow(bellui.Window)
}
