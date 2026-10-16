package editor

import (
	"context"
	"strings"
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// twoOverloads is a two-signature help result used by the overload tests.
func twoOverloads() *SignatureHelp {
	return &SignatureHelp{Signatures: []SignatureInformation{
		{Label: "f(a)", Parameters: []ParameterInformation{{Label: "a"}}},
		{Label: "f(x, y)", Parameters: []ParameterInformation{{Label: "x"}, {Label: "y"}}},
	}}
}

// demoSignatures is a small signature table used across the controller tests.
func demoSignatures() map[string]SignatureInformation {
	return map[string]SignatureInformation{
		"max": {
			Label:      "max(x, y)",
			Parameters: []ParameterInformation{{Label: "x"}, {Label: "y"}},
		},
	}
}

func editorWithSignatures(t *testing.T, funcs map[string]SignatureInformation) *Editor {
	t.Helper()
	e := renderedPopoverEditor(t, "")
	e.SetSignatureHelpProvider(&FuncSignatureProvider{
		Funcs: funcs,
		Text:  func() string { return e.Text() },
	})
	return e
}

// activeParamText returns the bolded (active) parameter segment of the visible
// signature card, or "" if none.
func activeParamText(e *Editor) string {
	if e.signature.rich == nil {
		return ""
	}
	for _, seg := range e.signature.rich.Segments {
		if ts, ok := seg.(*widget.TextSegment); ok && ts.Style.TextStyle.Bold {
			return ts.Text
		}
	}
	return ""
}

func TestCallContext(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		row, col  int
		wantName  string
		wantParam int
		wantOK    bool
	}{
		{"first arg", "max(", 0, 4, "max", 0, true},
		{"first arg typed", "max(1", 0, 5, "max", 0, true},
		{"second arg", "max(1, ", 0, 7, "max", 1, true},
		{"third arg", "max(1, 2, ", 0, 10, "max", 2, true},
		{"nested inner", "max(min(", 0, 8, "min", 0, true},
		{"nested outer after close", "max(min(1), ", 0, 12, "max", 1, true},
		{"closed call", "max(1)", 0, 6, "", 0, false},
		{"not a call", "(1, 2", 0, 5, "", 0, false},
		{"outside any call", "abc", 0, 3, "", 0, false},
		{"multiline", "max(\n1, ", 1, 3, "max", 1, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			name, param, ok := callContext(tc.text, tc.row, tc.col)
			if name != tc.wantName || param != tc.wantParam || ok != tc.wantOK {
				t.Errorf("callContext(%q, %d, %d) = (%q, %d, %v), want (%q, %d, %v)",
					tc.text, tc.row, tc.col, name, param, ok, tc.wantName, tc.wantParam, tc.wantOK)
			}
		})
	}
}

func TestSplitSignature(t *testing.T) {
	sig := SignatureInformation{Label: "max(x, y)", Parameters: []ParameterInformation{{Label: "x"}, {Label: "y"}}}
	pre, act, post := splitSignature(sig, 1)
	if pre != "max(x, " || act != "y" || post != ")" {
		t.Errorf("split active=1 = (%q,%q,%q)", pre, act, post)
	}

	// Param name also present in the function name resolves to the real param.
	mad := SignatureInformation{Label: "mad(a, b)", Parameters: []ParameterInformation{{Label: "a"}, {Label: "b"}}}
	pre, act, post = splitSignature(mad, 0)
	if pre != "mad(" || act != "a" || post != ", b)" {
		t.Errorf("split should skip the 'a' in 'mad': (%q,%q,%q)", pre, act, post)
	}

	// Out-of-range active parameter highlights nothing.
	if pre, act, _ := splitSignature(SignatureInformation{Label: "f(x)", Parameters: sig.Parameters}, 5); pre != "f(x)" || act != "" {
		t.Errorf("out-of-range active should yield plain label, got pre=%q act=%q", pre, act)
	}

	// Offset form disambiguates a repeated name: highlight the second "x".
	off := SignatureInformation{
		Label: "f(x int, x2 int)",
		Parameters: []ParameterInformation{
			{Label: "x int", LabelStart: 2, LabelEnd: 7},
			{Label: "x2 int", LabelStart: 9, LabelEnd: 15},
		},
	}
	if pre, act, post := splitSignature(off, 1); pre != "f(x int, " || act != "x2 int" || post != ")" {
		t.Errorf("offset form should pick the 2nd param: (%q,%q,%q)", pre, act, post)
	}
}

func TestSignature_TriggerOnOpenParen(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithSignatures(t, demoSignatures())

	typeRunes(e, "max(")
	if !e.signature.visible() {
		t.Fatal("signature card should be visible after typing '('")
	}
	if got := activeParamText(e); got != "x" {
		t.Errorf("first parameter should be active, got %q", got)
	}
}

func TestSignature_AdvancesOnComma(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithSignatures(t, demoSignatures())

	typeRunes(e, "max(1,")
	if !e.signature.visible() {
		t.Fatal("signature card should be visible inside the call")
	}
	if got := activeParamText(e); got != "y" {
		t.Errorf("second parameter should be active after a comma, got %q", got)
	}
}

func TestSignature_DismissesWhenCallCloses(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithSignatures(t, demoSignatures())

	typeRunes(e, "max(1")
	if !e.signature.visible() {
		t.Fatal("card should be visible")
	}
	typeRunes(e, ")")
	if e.signature.visible() {
		t.Error("closing the call should dismiss the signature card")
	}
}

func TestSignature_UnknownFunctionShowsNothing(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithSignatures(t, demoSignatures())
	typeRunes(e, "nope(")
	if e.signature.visible() {
		t.Error("an unknown function should show no signature card")
	}
}

func TestSignature_NoProviderUnchanged(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "")
	typeRunes(e, "max(") // must not panic or show
	if e.signature.visible() {
		t.Error("no signature card without a provider")
	}
	if e.Text() != "max(" {
		t.Errorf("typing should behave normally, got %q", e.Text())
	}
}

func TestSignature_ClearingProviderDismisses(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithSignatures(t, demoSignatures())
	typeRunes(e, "max(")
	if !e.signature.visible() {
		t.Fatal("card should be visible")
	}
	e.SetSignatureHelpProvider(nil)
	if e.signature.visible() {
		t.Error("clearing the provider should dismiss the card")
	}
}

func TestSignature_YieldsToCompletion(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithSignatures(t, demoSignatures())
	e.SetCompletionProvider(&WordListCompletionProvider{Words: []string{"value", "values"}})

	typeRunes(e, "max(")
	if !e.signature.visible() {
		t.Fatal("signature card should be visible after '('")
	}
	// Typing an identifier opens completion, which takes the popover.
	typeRunes(e, "va")
	if !e.completion.visible() {
		t.Fatal("completion should open while typing an argument")
	}
	if e.signature.visible() {
		t.Error("signature must yield the popover to completion")
	}
}

// stubSignatureProvider returns a fixed SignatureHelp regardless of position.
type stubSignatureProvider struct{ help *SignatureHelp }

func (p *stubSignatureProvider) Signatures(_ context.Context, _, _ int) (*SignatureHelp, error) {
	return p.help, nil
}

func TestSignature_InvokeShowsAtCursor(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithSignatures(t, demoSignatures())

	// Cursor sits inside an existing call that was not just typed.
	e.SetText("max(1")
	e.SetCursorPos(0, 5)
	e.ShowSignatureHelpAtCursor()
	if !e.signature.visible() {
		t.Fatal("explicit invoke should show signature help inside a call")
	}
	if got := activeParamText(e); got != "x" {
		t.Errorf("first parameter should be active, got %q", got)
	}
}

func TestSignature_InvokeYieldsToCompletion(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithSignatures(t, demoSignatures())
	e.SetCompletionProvider(&WordListCompletionProvider{Words: []string{"value"}})

	typeRunes(e, "max(va") // completion is open for "va"
	if !e.completion.visible() {
		t.Fatal("completion should be open")
	}
	e.ShowSignatureHelpAtCursor()
	if e.signature.visible() {
		t.Error("invoke must not steal the popover from an open completion list")
	}
}

func TestSignature_CycleOverloads(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "")
	e.SetSignatureHelpProvider(&stubSignatureProvider{help: &SignatureHelp{
		Signatures: []SignatureInformation{
			{Label: "f(a)", Parameters: []ParameterInformation{{Label: "a"}}},
			{Label: "f(x, y)", Parameters: []ParameterInformation{{Label: "x"}, {Label: "y"}}},
		},
	}})

	typeRunes(e, "f(")
	if !e.signature.visible() {
		t.Fatal("card should be visible")
	}
	if e.signature.sig != 0 || activeParamText(e) != "a" {
		t.Fatalf("first overload should show, sig=%d active=%q", e.signature.sig, activeParamText(e))
	}

	e.signature.cycle(1)
	if e.signature.sig != 1 || activeParamText(e) != "x" {
		t.Errorf("after cycle should show 2nd overload, sig=%d active=%q", e.signature.sig, activeParamText(e))
	}

	e.signature.cycle(1) // wraps back to the first
	if e.signature.sig != 0 {
		t.Errorf("cycle should wrap to the first overload, got %d", e.signature.sig)
	}
}

func TestSignature_CycleNoOpForSingleSignature(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithSignatures(t, demoSignatures())
	typeRunes(e, "max(")
	if !e.signature.visible() {
		t.Fatal("card should be visible")
	}
	e.signature.cycle(1) // single signature → no change, stays visible
	if e.signature.sig != 0 || !e.signature.visible() {
		t.Errorf("cycling a single signature should be a no-op, sig=%d visible=%v",
			e.signature.sig, e.signature.visible())
	}
}

func TestSignature_PreservesOverloadWhileTyping(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "")
	e.SetSignatureHelpProvider(&stubSignatureProvider{help: twoOverloads()})

	typeRunes(e, "f(")
	e.signature.cycle(1)
	if e.signature.sig != 1 {
		t.Fatalf("manual cycle should select overload 1, got %d", e.signature.sig)
	}
	// Typing another argument char re-queries; the same call must keep the
	// manually-selected overload (pre-rebrand tracker #38).
	typeRunes(e, "a")
	if e.signature.sig != 1 {
		t.Errorf("manual overload should survive continued typing, got %d", e.signature.sig)
	}
}

func TestSignature_ResetsOverloadOnDifferentCall(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "")
	p := &stubSignatureProvider{help: twoOverloads()}
	e.SetSignatureHelpProvider(p)

	typeRunes(e, "f(")
	e.signature.cycle(1)
	if e.signature.sig != 1 {
		t.Fatalf("expected overload 1, got %d", e.signature.sig)
	}
	// A different signature set (a different call) resets to the provider default.
	p.help = &SignatureHelp{Signatures: []SignatureInformation{
		{Label: "g(z)", Parameters: []ParameterInformation{{Label: "z"}}},
	}}
	typeRunes(e, ",") // trigger a re-query
	if e.signature.sig != 0 {
		t.Errorf("a different call should reset the overload selection, got %d", e.signature.sig)
	}
}

func TestSignature_OverloadArrows(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "")
	e.SetSignatureHelpProvider(&stubSignatureProvider{help: twoOverloads()})

	typeRunes(e, "f(")
	if !e.signature.arrows.Visible() {
		t.Fatal("overload arrows should be visible for multiple signatures")
	}
	if e.signature.count.Text != "1/2" {
		t.Errorf("counter should read 1/2, got %q", e.signature.count.Text)
	}

	// The down arrow is the third child (up, count, down); tapping it cycles.
	down, ok := e.signature.arrows.Objects[2].(*sigArrow)
	if !ok {
		t.Fatalf("expected a sigArrow at index 2, got %T", e.signature.arrows.Objects[2])
	}
	down.Tapped(&event.PointEvent{})
	if e.signature.sig != 1 || e.signature.count.Text != "2/2" {
		t.Errorf("tapping the down arrow should advance to 2/2, sig=%d count=%q",
			e.signature.sig, e.signature.count.Text)
	}
}

func TestSignature_ArrowsHiddenForSingleSignature(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithSignatures(t, demoSignatures())
	typeRunes(e, "max(")
	if !e.signature.visible() {
		t.Fatal("card should be visible")
	}
	if e.signature.arrows.Visible() {
		t.Error("overload arrows should be hidden for a single signature")
	}
}

// sigCardText returns all text rendered in the signature card.
func sigCardText(e *Editor) string {
	var b strings.Builder
	for _, seg := range e.signature.rich.Segments {
		if ts, ok := seg.(*widget.TextSegment); ok {
			b.WriteString(ts.Text)
		}
	}
	return b.String()
}

func TestSignature_PlaintextDocumentation(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithSignatures(t, map[string]SignatureInformation{
		"f": {
			Label: "f(a)", Documentation: "see _ref_",
			Parameters: []ParameterInformation{{Label: "a"}},
		},
	})
	typeRunes(e, "f(")
	if !e.signature.visible() {
		t.Fatal("signature card should be visible")
	}
	if got := sigCardText(e); !strings.Contains(got, "_ref_") {
		t.Errorf("plaintext signature docs should render verbatim, got %q", got)
	}
}

func TestSignature_MarkdownDocumentation(t *testing.T) {
	test.NewTempApp(t)
	e := editorWithSignatures(t, map[string]SignatureInformation{
		"f": {
			Label: "f(a)", Documentation: "see _ref_", DocumentationMarkdown: true,
			Parameters: []ParameterInformation{{Label: "a"}},
		},
	})
	typeRunes(e, "f(")
	if !e.signature.visible() {
		t.Fatal("signature card should be visible")
	}
	if got := sigCardText(e); !strings.Contains(got, "ref") || strings.Contains(got, "_ref_") {
		t.Errorf("markdown signature docs should render formatted (underscores consumed), got %q", got)
	}
}

func TestFuncSignatureProvider(t *testing.T) {
	text := "max(1, 2)"
	p := &FuncSignatureProvider{
		Funcs: demoSignatures(),
		Text:  func() string { return text },
	}
	help, err := p.Signatures(context.Background(), 0, 7) // inside the 2nd arg
	if err != nil || help == nil {
		t.Fatalf("expected signature help, got %v (err %v)", help, err)
	}
	if help.active().Label != "max(x, y)" || help.ActiveParameter != 1 {
		t.Errorf("got label %q active %d, want \"max(x, y)\" active 1",
			help.active().Label, help.ActiveParameter)
	}

	// Outside any known call, the provider returns nothing.
	if got, _ := p.Signatures(context.Background(), 0, 9); got != nil {
		t.Errorf("cursor past the call should yield no help, got %v", got)
	}
}
