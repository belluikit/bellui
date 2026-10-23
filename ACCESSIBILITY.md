# Accessibility Roadmap

Accessibility gaps and planned improvements for the Bellui fork. The design,
invariants and per-platform detail live in `docs/notes/accessibility.md`.

## Status

The public semantics (`core/accessible.go`: `Accessible`, `AccessibleDescriber`,
`AccessibleValuer`, `AccessibleStateful`, `AccessibleActioner`, with the `Role*`,
`State*` and `Action*` constants) are implemented by most stock widgets (see
`widget/accessible_test.go`). Behind `-tags accessibility`, a bounds-aware walker
feeds five platform bridges. What remains is UX enforcement and the gaps listed
below.

## Remaining UX Work

- Focus indicator styling enforcement
- Keyboard shortcut help/documentation system
- Skip links / landmark navigation

## Missing Annotations & Semantics

- Alt-text for `canvas.Image` (no `Description` field)
- Form-level labeling (associating a Form item's label with its Entry for screen readers)
- `widget.Chip` delete button is a raw `canvas.Image` with no accessible role or
  delete action — invisible to screen readers

## Widgets Missing `core.Accessible` Implementation

- `widget.ContextMenu` — the wrapped content is exposed, but the decorator itself
  has no role and offers no action to open its menu, so the menu is reachable
  only by a secondary tap.

## Completed

- **Core type infrastructure** — `Accessible`, `AccessibleDescriber`, `AccessibleValuer`, `AccessibleStateful`, `AccessibleActioner` interfaces, role/state/action constants, generic `WalkAccessibleTree`.
- **Platform bridges** (`-tags accessibility`) — macOS NSAccessibility, Windows UIA, Linux AT-SPI2 (pure Go), Android virtual nodes, iOS. Verify with `hack/a11yprobe`.
- **Live region announcements** — `bellui.AnnounceToScreenReader`; `Toast` and `Snackbar` announce their text.
- **Date/time and input widgets** — `DatePicker`, `TimePicker`, `DateTimePicker` (RoleGroup with a locale-formatted value), `DateEntry` / `DateTimeEntry` (through the embedded `Entry`), `ScrollIndicator` (RoleScrollBar), `SearchField` (RoleSearchField), `SpinBox` (RoleSpinButton with value, range and increment/decrement actions).
