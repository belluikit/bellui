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
| [`x/mmedia`](mmedia) | Media playback widgets (video, audio, subtitles) | `trendvidia/fyplayer` |

All five are **migrated and passing** (32 test packages between them). None
carries a `replace` directive — every module resolves its dependencies from
published tags.

`x/dag` is a partial extraction — only `dagwidget` and its data dependency
`dag`; see its README. The other four are whole-repo moves.

`x/mmedia` is the only one with a non-Go dependency: FFmpeg, via
`belluikit/goffmpeg` and the per-platform `goffmpeg-libs` modules. It is
consequently the only one whose build is platform-gated, and the only one whose
tests must run under **both** painters — see its `PROVENANCE.md`.
