// Command wasm is a browser harness for exercising the *complete* fycoder
// editor under GOOS=js/GOARCH=wasm — the runtime half of the WASM/web-target
// spike (docs/roadmap/10-wasm-web-target.md) and the acceptance harness for
// trendvidia/fyne#376.
//
// Compilation is already proven; this harness validates the behaviours that can
// only be seen in a real browser, one button each:
//
//   - Large-file load + virtualisation — a 50k-line document renders and scrolls
//     with row virtualisation active (Load 50k).
//   - Folding — the FoldGutter chevrons collapse a block; the placeholder shows
//     on the header and the external gutter + decorations stay aligned (Fold all
//     / Unfold all, or click a chevron).
//   - AutoFold — the PARAMS block loads pre-collapsed; the 50k file auto-folds
//     its giant interior on load.
//   - Protected ranges — typing/paste/IME-commit into the protected line is
//     vetoed (Phase 4); the status line reports the attempt.
//   - Caret-opaque redaction — the masked API key shows an opaque bar and bars
//     the caret (Toggle mask).
//
// Build & serve:  make serve   (then open http://localhost:8080)
package main

import (
	"fmt"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/editor"
	"github.com/belluikit/bellui/x/editor/highlight/starlark"
)

// sample is a small strategy script with a secret to redact, a pre-collapsed
// PARAMS block (AutoFold), and a protected line.
const sample = `# Strategy: momentum_scalper
#
# Buys when price momentum exceeds threshold and sells on reversal.

META = {
    "version": "1.0",
    "use_engine": True,
    "api_key": "sk-live-7Qd9f2KpZ1xR8vN3",
}

PARAMS = {
    "entry_threshold": 0.15,
    "exit_threshold": -0.05,
    "max_hold_games": 3,
}

def evaluate(ctx):
    tick = ctx.tick
    score = ctx.score
    mid = tick.mid

    # Entry logic: buy when momentum exceeds threshold.
    if score.momentum > PARAMS["entry_threshold"]:
        return {"action": "buy", "price": mid + 0.02}

    return {"action": "hold"}
`

// bigDoc builds an n-line document for large-file scroll/virtualisation testing.
func bigDoc(n int) string {
	var b strings.Builder
	b.Grow(n * 48)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "def rule_%05d(ctx):  # generated rule %d — windowed to the viewport, not the file\n", i, i)
	}
	return b.String()
}

// spanOf locates the first occurrence of substr and returns its (row,col) range.
func spanOf(text, substr string) (editor.TextRange, bool) {
	for row, line := range strings.Split(text, "\n") {
		if idx := strings.Index(line, substr); idx >= 0 {
			startCol := len([]rune(line[:idx]))
			return editor.TextRange{
				StartRow: row, StartCol: startCol,
				EndRow: row, EndCol: startCol + len([]rune(substr)),
			}, true
		}
	}
	return editor.TextRange{}, false
}

// lineSpan returns the whole-line range for the line containing substr.
func lineSpan(text, substr string) (editor.TextRange, bool) {
	for row, line := range strings.Split(text, "\n") {
		if strings.Contains(line, substr) {
			return editor.TextRange{StartRow: row, StartCol: 0, EndRow: row, EndCol: len([]rune(line))}, true
		}
	}
	return editor.TextRange{}, false
}

// sampleFolds derives foldable regions from the sample content: the header
// comment block, the META and PARAMS dict literals (PARAMS pre-collapsed with a
// summary so AutoFold + the placeholder show on first paint), and evaluate().
func sampleFolds() []editor.FoldRegion {
	lines := strings.Split(sample, "\n")
	var regions []editor.FoldRegion

	if len(lines) > 0 && strings.HasPrefix(lines[0], "#") {
		end := 0
		for end+1 < len(lines) && strings.HasPrefix(lines[end+1], "#") {
			end++
		}
		if end > 0 {
			regions = append(regions, editor.FoldRegion{
				Range: editor.TextRange{StartRow: 0, EndRow: end},
				Kind:  editor.FoldKindComment,
			})
		}
	}

	dict := func(header string, collapsed bool, summary string) {
		start := -1
		for i, l := range lines {
			if strings.HasPrefix(l, header) {
				start = i
				break
			}
		}
		if start < 0 {
			return
		}
		for i := start + 1; i < len(lines); i++ {
			if strings.HasPrefix(lines[i], "}") {
				regions = append(regions, editor.FoldRegion{
					Range:     editor.TextRange{StartRow: start, EndRow: i},
					Kind:      editor.FoldKindBlock,
					Collapsed: collapsed,
					Summary:   summary,
				})
				return
			}
		}
	}
	dict("META = {", false, "")
	dict("PARAMS = {", true, " ... 3 params") // pre-collapsed → AutoFold demo

	for i, l := range lines {
		if strings.HasPrefix(l, "def evaluate") {
			end := len(lines) - 1
			for end > i && strings.TrimSpace(lines[end]) == "" {
				end--
			}
			if end > i {
				regions = append(regions, editor.FoldRegion{
					Range: editor.TextRange{StartRow: i, EndRow: end},
					Kind:  editor.FoldKindBlock,
				})
			}
			break
		}
	}
	return regions
}

func main() {
	a := app.New()
	w := a.NewWindow("fycoder — wasm F1–F5 acceptance harness")

	status := widget.NewLabel("Ready — cursor 1,1")
	status.TextStyle = bellui.TextStyle{Monospace: true}

	ed := editor.NewEditor()
	ed.Highlighter = starlark.New()
	// Composite gutter: line numbers + Phase 5 fold chevrons.
	ed.Gutter = editor.NewCompositeGutter(
		editor.NewLineNumberGutter(),
		ed.FoldGutter(),
	)

	ed.OnCursorChanged = func(row, col int) {
		status.SetText(fmt.Sprintf("cursor %d,%d", row+1, col+1))
	}

	ed.SetText(sample)

	// Phase 4 protected range: make the api_key line read-only. Any edit that
	// touches it is vetoed and reported.
	ed.SetProtectedRanges(protectedFromSample())
	ed.OnProtectedEditAttempt(func(r editor.TextRange) {
		status.SetText(fmt.Sprintf("VETOED: line %d is protected (read-only)", r.StartRow+1))
	})

	// Phase 4 redaction: mask the secret value for real — opaque bar + caret bar
	// + atomic edit veto.
	redact := func() {
		if r, ok := spanOf(sample, "sk-live-7Qd9f2KpZ1xR8vN3"); ok {
			ed.SetRedactions([]editor.TextRange{r})
		}
	}
	redact()

	// Phase 5 folding: register the sample's regions (PARAMS pre-collapsed).
	folds := sampleFolds()
	ed.SetFoldRegions(folds)

	// Controls.
	foldAll := widget.NewButton("Fold all", func() {
		ed.FoldAll()
		status.SetText("folded all regions")
	})
	unfoldAll := widget.NewButton("Unfold all", func() {
		ed.UnfoldAll()
		status.SetText("unfolded all regions")
	})

	masked := true
	var maskBtn *widget.Button
	maskBtn = widget.NewButton("Reveal secret", func() {
		masked = !masked
		if masked {
			ed.SetRedactionMode(editor.Masked)
			maskBtn.SetText("Reveal secret")
			status.SetText("redaction MASKED (bar + caret barred)")
		} else {
			ed.SetRedactionMode(editor.Revealed)
			maskBtn.SetText("Mask secret")
			status.SetText("redaction REVEALED")
		}
	})

	const bigLines = 50000
	showingBig := false
	var bigBtn *widget.Button
	bigBtn = widget.NewButton(fmt.Sprintf("Load %d lines", bigLines), func() {
		if showingBig {
			ed.SetText(sample)
			ed.SetProtectedRanges(protectedFromSample())
			redact()
			ed.SetFoldRegions(folds)
			bigBtn.SetText(fmt.Sprintf("Load %d lines", bigLines))
			status.SetText("Ready — sample")
		} else {
			ed.SetText(bigDoc(bigLines))
			ed.SetProtectedRanges(nil)
			ed.SetRedactions(nil)
			// Leave the big file UNFOLDED so virtualised scrolling is demonstrable
			// (AutoFold is shown by the pre-collapsed PARAMS block in the sample).
			ed.SetFoldRegions(nil)
			bigBtn.SetText("Load sample")
			status.SetText(fmt.Sprintf("%d lines — row-virtualised; Ctrl+End / PageDown to page down", bigLines))
		}
		showingBig = !showingBig
	})

	// Deep scroll into the big file is now driven by the editor's own keyboard
	// paging (Ctrl+End / PageDown / PageUp / Ctrl+Home), which reaches the app on
	// wasm since glfw-js v1.0.2 (fyne#377/#397). The old "Jump to end" button — an
	// Editor.RevealRange workaround added when keyboard/wheel deep-scroll was
	// impossible in the browser (#118) — is retired (#120); RevealRange itself
	// stays exercised by the find-bar/reveal unit tests.
	toolbar := container.NewHBox(foldAll, unfoldAll, widget.NewSeparator(), maskBtn, bigBtn, widget.NewSeparator(), status)

	w.SetContent(container.NewBorder(toolbar, nil, nil, nil, ed))
	w.Resize(geo.NewSize(1000, 720))
	w.Show()
	ed.Focus()
	a.Run()
}

// protectedFromSample re-derives the api_key protected range for restore-on-reset.
func protectedFromSample() []editor.TextRange {
	if r, ok := lineSpan(sample, "api_key"); ok {
		return []editor.TextRange{r}
	}
	return nil
}
