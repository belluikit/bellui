# Bellui X

Extension libraries for the [Bellui](https://github.com/belluikit/bellui) toolkit.

Each subdirectory is a **separate Go module** with its own `go.mod`, so adding an
extension never widens the core toolkit's dependency graph:

| Module | Description | Migrated from |
| --- | --- | --- |
| [`x/charts`](charts) | Charting widgets | `trendvidia/fycharts` |
| [`x/tables`](tables) | Advanced table widgets | `trendvidia/fytables` |
| [`x/dag`](dag) | Directed-acyclic-graph widgets | `trendvidia/fydag` (`dagwidget` + `dag`) |
| [`x/editor`](editor) | Embeddable code editor (Bellui Editor) | `trendvidia/fycoder` |

All four are currently **scaffolds** — see each module's README for the tracking
issue. The `replace` directive pointing at the repo root is for local
development; it is dropped when a module is first tagged and published.
