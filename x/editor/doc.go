// Package fycoder provides a composable code editor widget for Bellui applications.
//
// The editor supports pluggable syntax highlighting, a customizable gutter
// (line numbers, breakpoints, annotations), row-level decorations (current line
// highlight, search matches), code folding (collapse/expand blocks), and a
// built-in find bar.
//
// # Architecture
//
// The editor separates concerns into composable interfaces:
//
//   - [Highlighter] provides per-line syntax highlighting via [StyleRun] tokens.
//   - [GutterProvider] renders the area left of the text (line numbers, markers).
//   - [RowDecorator] adds background or overlay effects to individual text rows.
//
// The gutter is rendered as a separate widget outside the text scroll viewport,
// so it stays pinned when scrolling horizontally, matching the behavior of
// production code editors.
//
// # Quick Start
//
//	editor := editor.NewEditor()
//	editor.Highlighter = myHighlighter
//	editor.Gutter = editor.NewCompositeGutter(
//	    editor.NewLineNumberGutter(),
//	    editor.NewBreakpointGutter(onToggle),
//	)
//	window.SetContent(editor)
package editor
