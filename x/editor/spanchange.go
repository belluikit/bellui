package editor

import (
	"sort"
	"strings"
)

// SpanChangeKind classifies how a named span changed between an open-time
// snapshot and the current document.
type SpanChangeKind int

const (
	// SpanModified marks a span present in both snapshot and current whose value
	// differs.
	SpanModified SpanChangeKind = iota
	// SpanAdded marks a span present in the current document but not the snapshot.
	SpanAdded
	// SpanRemoved marks a span present in the snapshot but absent from the current
	// document.
	SpanRemoved
)

// NamedSpan is a current document span keyed by a stable name (e.g. a secret's
// field name). The name — not the range — is what ties a span to its snapshot
// value, so a span that moved is still matched.
type NamedSpan struct {
	Name  string
	Range TextRange
}

// SpanChange reports one named span's classification.
type SpanChange struct {
	Name string
	Kind SpanChangeKind
}

// ClassifySpanChanges diffs an open-time snapshot of span values (name → value)
// against the current document, reading each current span's value out of the
// current text. It is a pure helper with no Fyne dependency — a port of pxfed's
// secret-changes.ts, hoisted here so any host gets the save-confirm diff for
// free.
//
// Three classes surface:
//   - SpanModified — name in both snapshot and spans, value differs.
//   - SpanAdded    — name in spans but not the snapshot.
//   - SpanRemoved  — name in the snapshot but not in spans.
//
// Unchanged spans produce no entry. Modified/added entries keep the order of
// spans; removed entries follow, ordered by name (Go map iteration is
// unordered, so removed names are sorted for a deterministic result).
func ClassifySpanChanges(snapshot map[string]string, current string, spans []NamedSpan) []SpanChange {
	out := []SpanChange{}
	seen := make(map[string]bool, len(spans))
	for _, s := range spans {
		seen[s.Name] = true
		cur := textInRange(current, s.Range)
		orig, ok := snapshot[s.Name]
		switch {
		case !ok:
			out = append(out, SpanChange{Name: s.Name, Kind: SpanAdded})
		case orig != cur:
			out = append(out, SpanChange{Name: s.Name, Kind: SpanModified})
		}
	}

	var removed []string
	for name := range snapshot {
		if !seen[name] {
			removed = append(removed, name)
		}
	}
	sort.Strings(removed)
	for _, name := range removed {
		out = append(out, SpanChange{Name: name, Kind: SpanRemoved})
	}
	return out
}

// textInRange returns the substring of text covered by the half-open (row, col)
// range r, in rune-column units (matching the rest of fycoder's addressing). A
// multi-row range joins the spanned line fragments with "\n". Out-of-range rows
// or columns are clamped; an empty range yields "".
func textInRange(text string, r TextRange) string {
	if r.IsEmpty() {
		return ""
	}
	lines := strings.Split(text, "\n")
	if r.StartRow < 0 || r.StartRow >= len(lines) {
		return ""
	}
	endRow := r.EndRow
	if endRow >= len(lines) {
		endRow = len(lines) - 1
	}

	clampCol := func(line string, col int) int {
		n := len([]rune(line))
		if col < 0 {
			return 0
		}
		if col > n {
			return n
		}
		return col
	}

	if r.StartRow == endRow {
		runes := []rune(lines[r.StartRow])
		start := clampCol(lines[r.StartRow], r.StartCol)
		end := clampCol(lines[r.StartRow], r.EndCol)
		if end < start {
			return ""
		}
		return string(runes[start:end])
	}

	var b strings.Builder
	// First row: from StartCol to end of line.
	first := []rune(lines[r.StartRow])
	b.WriteString(string(first[clampCol(lines[r.StartRow], r.StartCol):]))
	// Interior rows: whole line.
	for row := r.StartRow + 1; row < endRow; row++ {
		b.WriteByte('\n')
		b.WriteString(lines[row])
	}
	// Last row: from start of line to EndCol.
	b.WriteByte('\n')
	last := []rune(lines[endRow])
	b.WriteString(string(last[:clampCol(lines[endRow], r.EndCol)]))
	return b.String()
}
