package binding

import (
	"sync"

	"github.com/belluikit/bellui/lang"
)

// localizedString is a String binding whose value is the translation of a
// message key. It re-derives that translation whenever the active locale changes
// via lang.SetLocale, and — for the plural variants — whenever its count source
// changes, pushing the new text to any bound widget.
//
// It only subscribes to locale (and count) changes while it actually has
// listeners (i.e. while bound to a widget), and unsubscribes again when the last
// one is removed, so a transient localized binding does not leak a subscription.
type localizedString struct {
	String

	key      string
	fallback string
	data     []any
	count    Int // nil unless this is a plural binding

	mu        sync.Mutex
	listeners int
	cancel    func()       // removes the lang locale-change subscription
	countL    DataListener // listens to the count source (plural only)
}

// NewLocalizedString returns a String binding whose value is the translation of
// key (resolved via [lang.Localize], i.e. the same lookup as lang.L) and which
// re-derives that translation whenever the active locale changes through
// [lang.SetLocale]. Bind it to a widget — e.g. with
// [github.com/belluikit/bellui/widget.NewLabelWithData] — and the widget's
// text follows a live language switch without a manual rebuild.
//
// Any template data is forwarded to lang.Localize on every (re-)evaluation, so
// pass values, not pre-formatted strings, for them to localize correctly.
//
// Since: 2.8
func NewLocalizedString(key string, data ...any) String {
	return newLocalized(key, key, nil, data)
}

// NewLocalizedStringWithKey returns a String binding whose value is the
// translation looked up by key, falling back to fallback when no translation is
// found (resolved via [lang.LocalizeKey], i.e. the same lookup as lang.X). Like
// [NewLocalizedString] it re-derives on a live [lang.SetLocale] change.
//
// Use this when your message IDs are distinct from the default text, the common
// pattern for larger translation catalogues.
//
// Since: 2.8
func NewLocalizedStringWithKey(key, fallback string, data ...any) String {
	return newLocalized(key, fallback, nil, data)
}

// NewLocalizedPlural returns a String binding whose value is the plural-form
// translation of key for the current value of count (resolved via
// [lang.LocalizePlural], i.e. the same lookup as lang.N). It re-derives whenever
// the active locale changes through [lang.SetLocale] or count changes, so both a
// language switch and a count change update a bound widget without a rebuild.
//
// count is an [Int] binding so the plural form tracks a live count; for a fixed
// count, pass a [NewInt] you Set once.
//
// Since: 2.8
func NewLocalizedPlural(key string, count Int, data ...any) String {
	return newLocalized(key, key, count, data)
}

// NewLocalizedPluralWithKey is the explicit-fallback form of
// [NewLocalizedPlural], resolving via [lang.LocalizePluralKey] (i.e. the same
// lookup as lang.XN).
//
// Since: 2.8
func NewLocalizedPluralWithKey(key, fallback string, count Int, data ...any) String {
	return newLocalized(key, fallback, count, data)
}

func newLocalized(key, fallback string, count Int, data []any) String {
	s := &localizedString{
		String:   NewString(),
		key:      key,
		fallback: fallback,
		data:     data,
		count:    count,
	}
	s.update() // resolve once so Get works before the binding is attached
	return s
}

// update re-evaluates the translation for the current locale (and count, for
// plurals) and pushes it to the underlying String, which only triggers listeners
// when the value changed.
func (s *localizedString) update() {
	if s.count == nil {
		_ = s.String.Set(lang.LocalizeKey(s.key, s.fallback, s.data...))
		return
	}

	n, err := s.count.Get()
	if err != nil {
		n = 0
	}
	data := s.data
	if len(data) == 0 {
		// Expose the live count to the template so the common case —
		// NewLocalizedPlural("Apples", count) with a "{{.Count}} apple(s)"
		// message — shows the current number, not a value frozen at
		// construction. A caller-supplied template data takes precedence.
		data = []any{map[string]any{"Count": n, "PluralCount": n}}
	}
	_ = s.String.Set(lang.LocalizePluralKey(s.key, s.fallback, n, data...))
}

// AddListener attaches l and, on the first listener, subscribes to locale (and
// count) changes so the translation tracks the active language while in use.
func (s *localizedString) AddListener(l DataListener) {
	s.mu.Lock()
	s.listeners++
	first := s.listeners == 1
	if first {
		s.cancel = lang.AddLocaleChangeListener(s.update)
		if s.count != nil && s.countL == nil {
			s.countL = NewDataListener(s.update)
		}
	}
	countL := s.countL
	s.mu.Unlock()

	if first {
		s.update() // catch a locale/count change between construction and binding
		if s.count != nil {
			s.count.AddListener(countL)
		}
	}
	s.String.AddListener(l)
}

// RemoveListener detaches l and, once the last listener is gone, drops the
// locale (and count) subscriptions so this binding can be garbage collected.
func (s *localizedString) RemoveListener(l DataListener) {
	s.String.RemoveListener(l)

	s.mu.Lock()
	if s.listeners > 0 {
		s.listeners--
	}
	last := s.listeners == 0
	cancel := s.cancel
	countL := s.countL
	if last {
		s.cancel = nil
	}
	s.mu.Unlock()

	if last {
		if cancel != nil {
			cancel()
		}
		if s.count != nil && countL != nil {
			s.count.RemoveListener(countL)
		}
	}
}
