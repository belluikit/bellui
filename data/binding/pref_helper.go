package binding

import (
	"sync"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/async"
)

type preferenceItem interface {
	checkForChange()
}

type preferenceBindings struct {
	async.Map[string, preferenceItem]
}

func (b *preferenceBindings) list() []preferenceItem {
	ret := []preferenceItem{}
	b.Range(func(_ string, item preferenceItem) bool {
		ret = append(ret, item)
		return true
	})
	return ret
}

type preferencesMap struct {
	prefs async.Map[bellui.Preferences, *preferenceBindings]

	appPrefs bellui.Preferences // the main application prefs, to check if it changed...
	appLock  sync.Mutex
}

func newPreferencesMap() *preferencesMap {
	return &preferencesMap{}
}

func (m *preferencesMap) ensurePreferencesAttached(p bellui.Preferences) *preferenceBindings {
	binds, loaded := m.prefs.LoadOrStore(p, &preferenceBindings{})
	if loaded {
		return binds
	}

	p.AddChangeListener(func() { m.preferencesChanged(p) })
	return binds
}

func (m *preferencesMap) getBindings(p bellui.Preferences) *preferenceBindings {
	if p == bellui.CurrentApp().Preferences() {
		m.appLock.Lock()
		prefs := m.appPrefs
		if m.appPrefs == nil {
			m.appPrefs = p
		}
		m.appLock.Unlock()
		if prefs != p {
			m.migratePreferences(prefs, p)
		}
	}
	binds, _ := m.prefs.Load(p)
	return binds
}

func (m *preferencesMap) preferencesChanged(p bellui.Preferences) {
	binds := m.getBindings(p)
	if binds == nil {
		return
	}
	for _, item := range binds.list() {
		item.checkForChange()
	}
}

// unbindPreference removes the binding for the given key from the given preferences,
// allowing it to be garbage collected.
func (m *preferencesMap) unbindPreference(key string, p bellui.Preferences) {
	binds, ok := m.prefs.Load(p)
	if !ok || binds == nil {
		return
	}
	binds.Delete(key)
}

// cleanupPreferences removes all bindings associated with the given preferences.
func (m *preferencesMap) cleanupPreferences(p bellui.Preferences) {
	m.prefs.Delete(p)
}

func (m *preferencesMap) migratePreferences(src, dst bellui.Preferences) {
	old, loaded := m.prefs.Load(src)
	if !loaded {
		return
	}

	m.prefs.Store(dst, old)
	m.prefs.Delete(src)
	m.appLock.Lock()
	m.appPrefs = dst
	m.appLock.Unlock()

	binds := m.getBindings(dst)
	if binds == nil {
		return
	}
	for _, b := range binds.list() {
		if backed, ok := b.(interface{ replaceProvider(bellui.Preferences) }); ok {
			backed.replaceProvider(dst)
		}
	}

	m.preferencesChanged(dst)
}
