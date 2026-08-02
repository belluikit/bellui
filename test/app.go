// Package test provides utility drivers for running UI tests without rendering to a screen.
package test // import "github.com/belluikit/bellui/test"

import (
	"net/url"
	"sync"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal"
	intapp "github.com/belluikit/bellui/internal/app"
	"github.com/belluikit/bellui/internal/async"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/internal/test"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
)

// ensure we have a dummy app loaded and ready to test
func init() {
	NewApp()
}

type app struct {
	driver       *driver
	settings     *testSettings
	prefs        bellui.Preferences
	propertyLock sync.RWMutex
	storage      bellui.Storage
	lifecycle    intapp.Lifecycle
	clip         bellui.Clipboard
	cloud        bellui.CloudProvider

	// user action variables
	appliedTheme            bellui.Theme
	lastNotification        *bellui.Notification
	lastNotificationOptions *bellui.NotificationOptions
}

func (a *app) CloudProvider() bellui.CloudProvider {
	return a.cloud
}

func (a *app) Icon() bellui.Resource {
	return nil
}

func (a *app) SetIcon(bellui.Resource) {
	// no-op
}

func (a *app) NewWindow(title string) bellui.Window {
	return a.driver.CreateWindow(title)
}

func (a *app) OpenURL(url *url.URL) error {
	// no-op
	return nil
}

func (a *app) Run() {
	// no-op
}

func (a *app) Quit() {
	// no-op
}

func (a *app) Clipboard() bellui.Clipboard {
	return a.clip
}

func (a *app) UniqueID() string {
	return "testApp" // TODO should this be randomised?
}

func (a *app) Driver() bellui.Driver {
	return a.driver
}

func (a *app) SendNotification(notify *bellui.Notification) {
	a.propertyLock.Lock()
	defer a.propertyLock.Unlock()

	a.lastNotification = notify
}

// SendNotificationWithOptions records the rich notification and its reduced plain
// form so tests can assert on either. Implements bellui.NotifierWithOptionsApp.
func (a *app) SendNotificationWithOptions(opts *bellui.NotificationOptions) {
	a.propertyLock.Lock()
	defer a.propertyLock.Unlock()

	a.lastNotificationOptions = opts
	if opts != nil {
		a.lastNotification = &bellui.Notification{Title: opts.Title, Content: opts.Content}
	}
}

func (a *app) SetCloudProvider(p bellui.CloudProvider) {
	if p == nil {
		a.cloud = nil
		return
	}

	a.transitionCloud(p)
}

func (a *app) Settings() bellui.Settings {
	return a.settings
}

func (a *app) Preferences() bellui.Preferences {
	return a.prefs
}

func (a *app) Storage() bellui.Storage {
	return a.storage
}

func (a *app) Lifecycle() bellui.Lifecycle {
	return &a.lifecycle
}

func (a *app) Metadata() bellui.AppMetadata {
	return bellui.AppMetadata{} // just dummy data
}

func (a *app) lastAppliedTheme() bellui.Theme {
	a.propertyLock.Lock()
	defer a.propertyLock.Unlock()

	return a.appliedTheme
}

func (a *app) transitionCloud(p bellui.CloudProvider) {
	if a.cloud != nil {
		a.cloud.Cleanup(a)
	}

	err := p.Setup(a)
	if err != nil {
		bellui.LogError("Failed to set up cloud provider "+p.ProviderName(), err)
		return
	}
	a.cloud = p

	var listeners []func()
	if cl, ok := a.prefs.(interface{ ChangeListenersCopy() []func() }); ok {
		listeners = cl.ChangeListenersCopy()
	}
	if pp, ok := p.(bellui.CloudProviderPreferences); ok {
		a.prefs = pp.CloudPreferences(a)
	} else {
		a.prefs = internal.NewInMemoryPreferences()
	}
	if store, ok := p.(bellui.CloudProviderStorage); ok {
		a.storage = store.CloudStorage(a)
	} else {
		a.storage = &testStorage{}
	}

	for _, l := range listeners {
		a.prefs.AddChangeListener(l)
		l() // assume that preferences have changed because we replaced the provider
	}

	// after transition ensure settings listener is fired
	a.settings.apply()
}

// NewApp returns a new dummy app used for testing.
// It loads a test driver which creates a virtual window in memory for testing.
func NewApp() bellui.App {
	settings := &testSettings{scale: 1.0, theme: Theme()}
	prefs := internal.NewInMemoryPreferences()
	store := &testStorage{}
	test := &app{settings: settings, prefs: prefs, storage: store, driver: NewDriver().(*driver), clip: NewClipboard()}
	settings.app = test
	root, _ := store.docRootURI()
	store.Docs = &internal.Docs{RootDocURI: root}
	painter.ClearFontCache()
	cache.ResetThemeCaches()
	bellui.SetCurrentApp(test)

	// Re-apply settings on a runtime layout-direction or locale change, mirroring
	// the real app so the refresh paths are exercised in tests.
	bellui.RegisterLayoutDirectionRefresh(func() { settings.apply() })
	lang.RegisterLocaleChangeRefresh(func() { settings.apply() })

	return test
}

// Declare conformity with the optional primary color settings interface
var _ bellui.PrimaryColorSettings = (*testSettings)(nil)

type testSettings struct {
	primaryColor string
	scale        float32
	theme        bellui.Theme
	variant      bellui.ThemeVariant
	variantSet   bool

	listeners       []func(bellui.Settings)
	changeListeners []chan bellui.Settings
	propertyLock    sync.RWMutex
	app             *app
}

func (s *testSettings) AddChangeListener(listener chan bellui.Settings) {
	s.propertyLock.Lock()
	defer s.propertyLock.Unlock()
	s.changeListeners = append(s.changeListeners, listener)
}

func (s *testSettings) AddListener(listener func(bellui.Settings)) {
	s.propertyLock.Lock()
	defer s.propertyLock.Unlock()
	s.listeners = append(s.listeners, listener)
}

func (s *testSettings) BuildType() bellui.BuildType {
	return bellui.BuildStandard
}

func (s *testSettings) PrimaryColor() string {
	if s.primaryColor != "" {
		return s.primaryColor
	}

	return theme.ColorBlue
}

// SetPrimaryColor sets the named primary color preference of the test app,
// mirroring the real app settings for tests of primary color switching.
func (s *testSettings) SetPrimaryColor(name string) {
	s.propertyLock.Lock()
	s.primaryColor = name
	s.propertyLock.Unlock()

	s.apply()
}

func (s *testSettings) SetTheme(theme bellui.Theme) {
	s.propertyLock.Lock()
	s.theme = theme
	s.propertyLock.Unlock()

	s.apply()
}

func (s *testSettings) ShowAnimations() bool {
	return true
}

func (s *testSettings) Theme() bellui.Theme {
	s.propertyLock.RLock()
	defer s.propertyLock.RUnlock()

	if s.theme == nil {
		return test.DarkTheme(theme.DefaultTheme())
	}

	return s.theme
}

func (s *testSettings) ThemeVariant() bellui.ThemeVariant {
	s.propertyLock.RLock()
	defer s.propertyLock.RUnlock()

	if s.variantSet {
		return s.variant
	}
	return 2 // not a preference
}

func (s *testSettings) SetThemeVariant(v bellui.ThemeVariant) {
	s.propertyLock.Lock()
	s.variant = v
	s.variantSet = true
	s.propertyLock.Unlock()

	s.apply()
}

func (s *testSettings) Scale() float32 {
	s.propertyLock.RLock()
	defer s.propertyLock.RUnlock()
	return s.scale
}

func (s *testSettings) apply() {
	s.propertyLock.RLock()
	listeners := s.changeListeners
	listenersFns := s.listeners
	s.propertyLock.RUnlock()

	for _, listener := range listeners {
		listener <- s
	}

	applyFn := func() {
		s.app.propertyLock.Lock()
		painter.ClearFontCache()
		cache.ResetThemeCaches()
		intapp.ApplySettings(s, s.app)
		s.app.propertyLock.Unlock()

		for _, l := range listenersFns {
			l(s)
		}
	}
	// apply must have COMPLETED the re-theme walk before it returns —
	// appliedTheme below is what waitForTheme polls, so an apply that is
	// still in flight makes every ApplyTheme-then-capture sequence a race.
	// Off the main goroutine the test driver's DoFromGoroutine runs inline,
	// which is synchronous already. ON the main goroutine (a suite driving a
	// real driver loop, like the local glfw one, applies themes from
	// runOnMain) it must not be used: its EnsureNotMain guard would punt the
	// walk to a bare goroutine and return immediately — the fire-and-forget
	// behind the #115 nightly flake ("Butto" captured mid-re-theme) and its
	// GUI-call-thread warning spam. Already being on the GUI goroutine is
	// exactly the contract Do exists to establish, so call straight through.
	if async.IsMainGoroutine() {
		applyFn()
	} else {
		s.app.driver.DoFromGoroutine(applyFn, false)
	}

	s.app.propertyLock.Lock()
	s.app.appliedTheme = s.Theme()
	s.app.propertyLock.Unlock()
}
