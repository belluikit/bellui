package lang

// BindText connects a translation key to a string setter so the text follows the
// active locale. apply is called immediately with the current translation of key
// (resolved via [Localize], i.e. the same lookup as L) and again every time the
// active locale changes through [SetLocale], until the returned remove func is
// called.
//
// It is the binding-independent counterpart to
// [github.com/belluikit/bellui/data/binding.NewLocalizedString]: use it for
// text on surfaces that have no data binding, such as a button label, a window
// title or a menu item, so a live language switch updates them without a manual
// rebuild. (Widgets that accept a [github.com/belluikit/bellui/data/binding.String]
// — e.g. via NewLabelWithData — should prefer NewLocalizedString.)
//
// Example — a button label that re-translates on a live language switch:
//
//	btn := widget.NewButton("", onCancel)
//	remove := lang.BindText("Cancel", btn.SetText)
//	// ...later, when the button is no longer used:
//	remove()
//
// Example — a window title and a menu item:
//
//	lang.BindText("My Application", w.SetTitle)
//	lang.BindText("Open", func(s string) { item.Label = s; menu.Refresh() })
//
// Any template data is forwarded to [Localize] on every (re-)evaluation, so pass
// values, not pre-formatted strings, for them to localize correctly.
//
// apply runs on whichever goroutine calls [SetLocale] (documented to be the main
// one), matching the threading rules for updating widgets. Call remove when the
// target is gone so the subscription can be garbage collected; long-lived UI (a
// window title, a permanent toolbar button) may simply leave it for the app
// lifetime.
//
// Since: 2.8
func BindText(key string, apply func(string), data ...any) (remove func()) {
	apply(Localize(key, data...))
	return AddLocaleChangeListener(func() {
		apply(Localize(key, data...))
	})
}
