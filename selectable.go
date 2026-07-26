package bellui

// SelectableMode represents the window-level selectable mode which can override
// per-widget Selectable properties for display widgets.
//
// Since: 2.8
type SelectableMode int

const (
	// SelectablePerWidget uses each widget's own Selectable property to determine
	// whether its content can be selected and copied.
	SelectablePerWidget SelectableMode = iota
	// SelectableAll forces all display widgets in the window to be selectable,
	// regardless of their own Selectable property.
	SelectableAll
	// SelectableNone forces all display widgets in the window to be non-selectable,
	// regardless of their own Selectable property.
	SelectableNone
)
