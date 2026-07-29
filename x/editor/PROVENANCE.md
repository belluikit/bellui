# x/editor — consolidated from github.com/trendvidia/fycoder

Copied from `github.com/trendvidia/fycoder` @ `origin/main` @
`c766960ad85761d5fd6367f8bca621345a802795` (= tag `v0.12.0`, 2026-07-25).

Only mechanical rewrites were applied: the module path became
`github.com/belluikit/bellui/x/editor`, `package fycoder` became
`package editor`, and `github.com/trendvidia/fyne/v3` became
`github.com/belluikit/bellui`. Comments that name `fycoder` historically are
left as written. No API changed.

## Re-cut history

| Ref | Source | Why |
|---|---|---|
| `v1.0.0-rc.5` | `main` @ `c766960` (v0.12.0) | Corrects the below |
| `v1.0.0-rc.1`…`rc.4` | `chore/fyne-v3` @ fork point ≈ v0.6.1-rc.1 | **Wrong source** |

The original consolidation was cut from the source repo's `chore/fyne-v3`
branch rather than `main` (belluikit/bellui#85). That branch was an abandoned
duplicate — the real fyne-v3 migration had merged to `main` as #172 weeks
earlier and development continued there — so it sat 14 commits behind and the
published module lost, among other things, `SemanticToken` /
`SetSemanticTokens` (LSP semantic-token highlighting) and `accessibility.go`
(`SetAccessibilityLabel`, exposing the editor as a text area).

**The lesson this file exists to enforce:** record the ref *and* the sha, not a
version. The stale content corresponded to no tag at all, which is why every
attempt to identify it by version number failed. Verify the source is
`origin/main` before copying — a branch whose name matches what you are looking
for is not evidence that it is current.
