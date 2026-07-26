package editor

import (
	"testing"

	"github.com/belluikit/bellui/test"
)

func sampleDiagnostics() []Diagnostic {
	return []Diagnostic{
		{Range: TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 3}, Severity: SeverityError, Message: "bad", Source: "test"},
		{Range: TextRange{StartRow: 1, StartCol: 1, EndRow: 1, EndCol: 4}, Severity: SeverityWarning, Message: "iffy", Source: "test"},
	}
}

func TestDiagnostics_SetAndGet(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText("abc\ndefg")

	if got := e.Diagnostics(); got != nil {
		t.Fatalf("a fresh editor should have no diagnostics, got %v", got)
	}
	diags := sampleDiagnostics()
	e.SetDiagnostics(diags)
	if got := e.Diagnostics(); len(got) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(got))
	}
}

func TestDiagnostics_OnChangedFires(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()

	var got []Diagnostic
	calls := 0
	e.OnDiagnosticsChanged(func(d []Diagnostic) {
		got = d
		calls++
	})

	e.SetDiagnostics(sampleDiagnostics())
	if calls != 1 || len(got) != 2 {
		t.Fatalf("callback should fire once with 2 diagnostics, got calls=%d len=%d", calls, len(got))
	}

	// Clearing also fires, with an empty set.
	e.SetDiagnostics(nil)
	if calls != 2 || len(got) != 0 {
		t.Errorf("clearing should fire the callback with no diagnostics, got calls=%d len=%d", calls, len(got))
	}
}

func TestDiagnostics_Squiggles(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetDiagnostics(sampleDiagnostics())

	spans := e.diagnostics.Spans()
	if len(spans) != 2 {
		t.Fatalf("expected one span per diagnostic, got %d", len(spans))
	}
	for i, s := range spans {
		if s.Kind != SpanUnderline || s.UnderlineStyle != UnderlineWavy {
			t.Errorf("span %d should be a wavy underline, got kind=%v style=%v", i, s.Kind, s.UnderlineStyle)
		}
		if s.Color == nil {
			t.Errorf("span %d should be colored by severity", i)
		}
	}
	// Severity drives color: error and warning differ.
	if spans[0].Color == spans[1].Color {
		t.Error("error and warning squiggles should use different colors")
	}
	// Ranges are carried through unchanged.
	if spans[0].Range != (TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 3}) {
		t.Errorf("span range should match the diagnostic, got %v", spans[0].Range)
	}
}

func TestDiagnostics_ClearRemovesSquiggles(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetDiagnostics(sampleDiagnostics())
	if len(e.diagnostics.Spans()) == 0 {
		t.Fatal("expected squiggles after SetDiagnostics")
	}
	e.SetDiagnostics(nil)
	if got := e.diagnostics.Spans(); got != nil {
		t.Errorf("clearing diagnostics should remove all squiggles, got %v", got)
	}
}

func TestSeverityColorName(t *testing.T) {
	cases := map[Severity]string{
		SeverityError:   string(ColorNameDiagnosticError),
		SeverityWarning: string(ColorNameDiagnosticWarning),
		SeverityInfo:    string(ColorNameDiagnosticInfo),
		SeverityHint:    string(ColorNameDiagnosticHint),
	}
	for sev, want := range cases {
		if got := string(severityColorName(sev)); got != want {
			t.Errorf("severity %d → %q, want %q", sev, got, want)
		}
	}
	// All four severities resolve to distinct theme colors.
	seen := map[any]bool{}
	for sev := range cases {
		c := themeColor(severityColorName(sev))
		if seen[c] {
			t.Errorf("severity %d shares a color with another severity", sev)
		}
		seen[c] = true
	}
}

func TestDiagnostics_NoProviderUnchanged(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	// A bare editor never set diagnostics: the overlay draws nothing.
	if got := e.diagnostics.Spans(); got != nil {
		t.Errorf("a bare editor should draw no squiggles, got %v", got)
	}
}
