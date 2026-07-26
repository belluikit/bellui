# Accessibility Roadmap

Accessibility gaps and planned improvements for the Bellui fork.

## Status

Core type infrastructure is done (`Accessible`, `AccessibleDescriber`, `AccessibleValuer`, `AccessibleStateful`, `AccessibleActioner` interfaces, 47 roles, 9 state flags, 8 action constants, 36 widgets annotated). What remains is platform integration, UX enforcement, and widget coverage.

## Platform Bridges

- Platform accessibility bridges (NSAccessibility on macOS, ATK/AT-SPI on Linux, UIA on Windows)
- Live region announcements (for toasts, status updates, dynamic content)
- Focus indicator styling enforcement
- Keyboard shortcut help/documentation system
- Skip links / landmark navigation

## Missing Annotations & Semantics

- Alt-text for `canvas.Image` (no `Description` field)
- Form-level labeling (associating a Form item's label with its Entry for screen readers)
- `widget.Chip` delete button is a raw `canvas.Image` with no accessible role — invisible to screen readers

## Widgets Missing `core.Accessible` Implementation

Identified during full codebase audit (2026-04-19). These are all interactive, user-facing widgets that should expose accessible roles, labels, and values.

- `widget.DatePicker` / `widget.DateEntry`
- `widget.TimePicker` / `widget.DateTimePicker` / `widget.DateTimeEntry`
- `widget.ContextMenu`
- `widget.ScrollIndicator`
- `widget.SearchField`
- `widget.SpinBox`

## Completed

- **Core type infrastructure** — `Accessible`, `AccessibleDescriber`, `AccessibleValuer`, `AccessibleStateful`, `AccessibleActioner` interfaces. 47 roles, 9 state flags, 8 action constants. 36 widgets annotated. Generic `WalkAccessibleTree`.
