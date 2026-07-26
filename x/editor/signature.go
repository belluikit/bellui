package editor

import (
	"context"
	"strconv"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// ParameterInformation describes one parameter of a [SignatureInformation].
//
// The parameter's span within the signature label can be given two ways. When
// LabelEnd > LabelStart, those rune offsets locate the parameter exactly —
// preferred when a parameter name repeats or appears inside another parameter's
// type (the LSP offset form). Otherwise the editor locates Label as a substring.
type ParameterInformation struct {
	// Label is the parameter's substring within the signature's Label (e.g.
	// "a int"). The editor highlights this substring when the parameter is
	// active. Repeated labels resolve left-to-right.
	Label string
	// LabelStart and LabelEnd, when LabelEnd > LabelStart, give the [start, end)
	// rune offsets of this parameter within the signature label, taking
	// precedence over substring matching on Label.
	LabelStart, LabelEnd int
	// Documentation is optional help text shown while this parameter is active.
	// It is plain text unless DocumentationMarkdown is set.
	Documentation string
	// DocumentationMarkdown renders Documentation as markdown rather than plain
	// text (see [MarkupKind]). Defaults to plain text.
	DocumentationMarkdown bool
}

// SignatureInformation describes a single callable signature.
type SignatureInformation struct {
	// Label is the full signature text shown in the popover, e.g.
	// "foo(a int, b string) error".
	Label string
	// Documentation is optional help text shown beneath the signature. It is plain
	// text unless DocumentationMarkdown is set.
	Documentation string
	// DocumentationMarkdown renders Documentation as markdown rather than plain
	// text (see [MarkupKind]). Defaults to plain text.
	DocumentationMarkdown bool
	// Parameters lists the signature's parameters in order.
	Parameters []ParameterInformation
}

// SignatureHelp is the result of a [SignatureHelpProvider] query: the candidate
// signatures plus which signature and parameter are currently active.
type SignatureHelp struct {
	// Signatures are the candidate signatures (usually one).
	Signatures []SignatureInformation
	// ActiveSignature indexes the signature to display. Out-of-range falls
	// back to the first signature.
	ActiveSignature int
	// ActiveParameter indexes the parameter to highlight within the active
	// signature. Out-of-range simply highlights nothing.
	ActiveParameter int
}

// active returns the signature to display, clamping ActiveSignature into range.
func (h *SignatureHelp) active() SignatureInformation {
	i := h.ActiveSignature
	if i < 0 || i >= len(h.Signatures) {
		i = 0
	}
	return h.Signatures[i]
}

// SignatureHelpProvider supplies call-signature help at a document position. It
// is optional and LSP-shaped but carries no LSP types — a host can back it with
// a static table, ctags, or a language-server bridge.
//
// Signatures is called on the UI goroutine as the user types inside a call
// (refreshed on "(" and ",") and should return promptly. Return nil to show
// nothing — for example when the cursor is not inside a known call.
type SignatureHelpProvider interface {
	Signatures(ctx context.Context, row, col int) (*SignatureHelp, error)
}

// SetSignatureHelpProvider installs (or clears, with nil) the signature-help
// provider. With no provider set, editor behavior is unchanged.
func (e *Editor) SetSignatureHelpProvider(p SignatureHelpProvider) {
	e.signature.provider = p
	if p == nil {
		e.signature.dismiss()
	}
}

// ShowSignatureHelpAtCursor explicitly requests signature help at the current
// cursor position, independent of the "(" / "," trigger characters — bound to
// Ctrl+Shift+Space. It is a no-op when no [SignatureHelpProvider] is set or a
// completion list is open. While the card is shown, Alt+Down / Alt+Up cycle
// between overloads.
func (e *Editor) ShowSignatureHelpAtCursor() {
	if e.signature != nil {
		e.signature.invoke()
	}
}

// maxSignatureWidth caps the signature popover width; longer labels wrap.
const maxSignatureWidth = 520

// FuncSignatureProvider is a simple [SignatureHelpProvider] that looks up the
// enclosing function call in a fixed table and reports the active parameter from
// the cursor's position among the call's arguments — handy for demos, tests, and
// builtin/keyword signature dictionaries.
type FuncSignatureProvider struct {
	// Funcs maps a function name to its signature.
	Funcs map[string]SignatureInformation
	// Text returns the current document text, used to find the enclosing call.
	Text func() string
}

// Signatures finds the call enclosing (row, col) and, if its name is known,
// returns its signature with the active parameter set to the argument index at
// the cursor.
func (p *FuncSignatureProvider) Signatures(_ context.Context, row, col int) (*SignatureHelp, error) {
	if p.Text == nil {
		return nil, nil
	}
	name, param, ok := callContext(p.Text(), row, col)
	if !ok {
		return nil, nil
	}
	sig, found := p.Funcs[name]
	if !found {
		return nil, nil
	}
	return &SignatureHelp{
		Signatures:      []SignatureInformation{sig},
		ActiveParameter: param,
	}, nil
}

// callContext scans backward from (row, col) to find the innermost enclosing
// function call: the identifier immediately before an unmatched "(". It returns
// the function name and the zero-based index of the active argument (the count
// of top-level commas between that "(" and the cursor). ok is false when the
// cursor is not inside such a call.
func callContext(text string, row, col int) (name string, param int, ok bool) {
	runes := []rune(text)
	cur := rowColToOffset(text, row, col)
	if cur > len(runes) {
		cur = len(runes)
	}

	depth := 0
	open := -1
	for i := cur - 1; i >= 0; i-- {
		switch runes[i] {
		case ')':
			depth++
		case '(':
			if depth == 0 {
				open = i
			} else {
				depth--
			}
		case ',':
			if depth == 0 {
				param++
			}
		}
		if open >= 0 {
			break
		}
	}
	if open < 0 {
		return "", 0, false
	}

	// The identifier immediately before "(" (skipping spaces/tabs) names the call.
	j := open - 1
	for j >= 0 && (runes[j] == ' ' || runes[j] == '\t') {
		j--
	}
	end := j + 1
	for j >= 0 && isIdentRune(runes[j]) {
		j--
	}
	start := j + 1
	if start == end {
		return "", 0, false // a grouping paren, not a call
	}
	return string(runes[start:end]), param, true
}

// splitSignature splits the signature label into the text before the active
// parameter, the active parameter span, and the text after. When there is no
// active parameter (or its span cannot be located) the whole label is returned
// as the prefix with empty active/suffix.
func splitSignature(sig SignatureInformation, active int) (pre, act, post string) {
	label, params := sig.Label, sig.Parameters
	if active < 0 || active >= len(params) {
		return label, "", ""
	}
	p := params[active]

	// Offset form takes precedence: it disambiguates repeated names exactly.
	if p.LabelEnd > p.LabelStart {
		runes := []rune(label)
		s, e := p.LabelStart, p.LabelEnd
		if s < 0 {
			s = 0
		}
		if e > len(runes) {
			e = len(runes)
		}
		if s >= e {
			return label, "", ""
		}
		return string(runes[:s]), string(runes[s:e]), string(runes[e:])
	}

	if p.Label == "" {
		return label, "", ""
	}
	// Substring form: locate each parameter label in order so repeated names
	// resolve left-to-right rather than always matching the first occurrence.
	// Start after the opening "(" so a parameter name that also appears in the
	// function name (the "a" in "max") doesn't match there.
	pos := 0
	if i := strings.IndexByte(label, '('); i >= 0 {
		pos = i + 1
	}
	for i := 0; i <= active; i++ {
		idx := strings.Index(label[pos:], params[i].Label)
		if idx < 0 {
			return label, "", "" // can't locate — show the label plain
		}
		s := pos + idx
		e := s + len(params[i].Label)
		if i == active {
			return label[:s], label[s:e], label[e:]
		}
		pos = e
	}
	return label, "", ""
}

// activeDoc returns the documentation to show beneath the signature, with its
// markup kind: the active parameter's docs when present, otherwise the
// signature's own docs.
func activeDoc(sig SignatureInformation, active int) (string, MarkupKind) {
	if active >= 0 && active < len(sig.Parameters) && sig.Parameters[active].Documentation != "" {
		p := sig.Parameters[active]
		return p.Documentation, boolMarkupKind(p.DocumentationMarkdown)
	}
	return sig.Documentation, boolMarkupKind(sig.DocumentationMarkdown)
}

// signatureController owns signature-help state: its rich-text popover and the
// query/refresh lifecycle driven by typing inside a call.
type signatureController struct {
	e        *Editor
	provider SignatureHelpProvider

	pop     *popover
	rich    *widget.RichText
	sizer   *fixedSizeLayout
	content *bellui.Container

	arrows *bellui.Container // overload control (▲ N/M ▾), shown when >1 signature
	count  *canvas.Text      // "N/M" overload counter

	help *SignatureHelp // the help currently displayed
	sig  int            // index into help.Signatures currently shown
}

func newSignatureController(e *Editor) *signatureController {
	return &signatureController{e: e}
}

// visible reports whether the signature popover is the editor's active popover.
func (s *signatureController) visible() bool {
	return s.pop != nil && s.e.popoverShown(s.pop)
}

// onTextChanged is called after a user edit. It (re)queries signature help when
// the user enters or moves within a call. The card is a non-interactive
// companion: it shows alongside an open completion list rather than yielding to
// it (#79), sitting above the line while completion sits below.
func (s *signatureController) onTextChanged() {
	if s.provider == nil {
		return
	}
	ch := s.e.charBeforeCursor()
	if s.isTriggerChar(ch) || s.visible() {
		s.query()
	}
}

// isTriggerChar reports whether r is a signature-help trigger character ("(" or
// ",") while a provider is installed.
func (s *signatureController) isTriggerChar(r rune) bool {
	return s.provider != nil && (r == '(' || r == ',')
}

// invoke explicitly requests signature help at the cursor (e.g. Ctrl+Shift+Space),
// independent of trigger characters.
func (s *signatureController) invoke() {
	if s.provider == nil {
		return
	}
	s.query()
}

// query asks the provider for help at the cursor and shows it, dismissing when
// the provider reports no signature (e.g. the cursor left the call).
func (s *signatureController) query() {
	row, col := s.e.entry.CursorRow, s.e.entry.CursorColumn
	help, err := s.provider.Signatures(context.Background(), row, col)
	if err != nil || help == nil || len(help.Signatures) == 0 {
		s.dismiss()
		return
	}
	// Preserve a manually-cycled overload while the same call stays in view, so
	// Alt+Down/Up survives continued typing of arguments. Reset to the provider's
	// choice only when the signature set actually changes (a different call).
	keep := s.visible() && sameSignatures(s.help, help)
	s.help = help
	if !keep {
		s.sig = help.ActiveSignature
	}
	if s.sig < 0 || s.sig >= len(help.Signatures) {
		s.sig = 0
	}
	s.show(row, col)
}

// sameSignatures reports whether two help results describe the same set of
// overloads (same count and labels), i.e. the same call.
func sameSignatures(a, b *SignatureHelp) bool {
	if a == nil || b == nil || len(a.Signatures) != len(b.Signatures) {
		return false
	}
	for i := range a.Signatures {
		if a.Signatures[i].Label != b.Signatures[i].Label {
			return false
		}
	}
	return true
}

// cycle moves to the next/previous overload (Alt+Down / Alt+Up) when the card is
// visible and the active signature has more than one overload.
func (s *signatureController) cycle(delta int) {
	if !s.visible() || s.help == nil || len(s.help.Signatures) <= 1 {
		return
	}
	n := len(s.help.Signatures)
	s.sig = (s.sig + delta + n) % n
	s.render()
	s.resize()
	s.e.repositionPopover()
}

// show renders the current help and anchors the popover at the cursor cell.
func (s *signatureController) show(row, col int) {
	s.ensureUI()
	s.render()
	s.resize()
	// Re-anchor at the current cursor each refresh so the popover tracks typing.
	s.e.showPopoverAt(s.pop, row, col)
}

// render lays the active signature into the card: the signature line with the
// active parameter bolded, any documentation beneath, and — when there is more
// than one overload — the ▲ N/M ▾ overload control on the right.
func (s *signatureController) render() {
	sig := s.help.Signatures[s.sig]
	active := s.help.ActiveParameter
	pre, act, post := splitSignature(sig, active)

	var segs []widget.RichTextSegment
	if act == "" {
		segs = append(segs, &widget.TextSegment{Text: sig.Label, Style: widget.RichTextStyle{Inline: true}})
	} else {
		segs = append(
			segs,
			&widget.TextSegment{Text: pre, Style: widget.RichTextStyle{Inline: true}},
			&widget.TextSegment{Text: act, Style: widget.RichTextStyle{
				Inline:    true,
				TextStyle: bellui.TextStyle{Bold: true},
				ColorName: theme.ColorNamePrimary,
			}},
			&widget.TextSegment{Text: post, Style: widget.RichTextStyle{Inline: true}},
		)
	}
	if doc, kind := activeDoc(sig, active); doc != "" {
		// A non-inline segment starts a new block beneath the signature line.
		// Markdown docs bring their own styling; plain text is shown dimmed.
		if kind == MarkupMarkdown {
			segs = append(segs, markupSegments(doc, MarkupMarkdown)...)
		} else {
			segs = append(segs, &widget.TextSegment{
				Text:  doc,
				Style: widget.RichTextStyle{ColorName: theme.ColorNamePlaceHolder},
			})
		}
	}
	s.rich.Segments = segs
	s.rich.Refresh()

	if n := len(s.help.Signatures); n > 1 {
		s.count.Text = strconv.Itoa(s.sig+1) + "/" + strconv.Itoa(n)
		s.count.Refresh()
		s.arrows.Show()
	} else {
		s.arrows.Hide()
	}
}

// ensureUI lazily builds the rich-text card, the overload control, and the
// popover.
func (s *signatureController) ensureUI() {
	if s.pop != nil {
		return
	}
	s.rich = widget.NewRichText()
	s.rich.Wrapping = bellui.TextWrapWord

	// Overload control: ▲ / N/M / ▾ stacked on the card's right. The arrows are
	// non-focusable tappables, so clicking them cycles overloads without pulling
	// keyboard focus off the Entry (the Entry sits under the popover and stays
	// focused), keeping the card open.
	s.count = canvas.NewText("", theme.Color(theme.ColorNamePlaceHolder))
	s.count.TextSize = theme.CaptionTextSize()
	s.count.Alignment = bellui.TextAlignCenter
	s.arrows = container.NewVBox(
		newSigArrow(theme.MenuDropUpIcon(), func() { s.cycle(-1) }),
		s.count,
		newSigArrow(theme.MenuDropDownIcon(), func() { s.cycle(1) }),
	)
	s.arrows.Hide()

	inner := container.NewBorder(nil, nil, nil, s.arrows, s.rich)
	s.sizer = &fixedSizeLayout{}
	s.content = container.New(s.sizer, inner)
	s.pop = newPopover(s.content)
	s.pop.maxWidth = maxSignatureWidth
	// A non-interactive companion: it shows above the line so it sits clear of a
	// below-anchored completion list, and it does not capture keys, so typing,
	// arrows, and acceptance pass straight to the Entry; Escape dismisses via the
	// editor.
	s.pop.prefersAbove = true
}

// resize sizes the card to its wrapped content, capped at maxSignatureWidth,
// leaving room for the overload control when it is shown.
func (s *signatureController) resize() {
	var arrowsW float32
	multi := s.help != nil && len(s.help.Signatures) > 1
	if multi {
		arrowsW = s.arrows.MinSize().Width + theme.Padding()
	}

	natural := naturalRichSize(s.rich) // unwrapped width; see naturalRichSize
	w := natural.Width + arrowsW
	if w > maxSignatureWidth {
		w = maxSignatureWidth
	}
	// Constrain the text width so wrapped height is measured at the final width.
	richW := w - arrowsW
	if richW < 1 {
		richW = 1
	}
	s.rich.Resize(geo.NewSize(richW, natural.Height))
	height := s.rich.MinSize().Height
	if multi {
		if ah := s.arrows.MinSize().Height; ah > height {
			height = ah
		}
	}
	s.sizer.size = geo.NewSize(w, height)
	s.content.Refresh()
}

// dismiss hides the signature popover if shown and clears the cached help.
func (s *signatureController) dismiss() {
	if s.visible() {
		s.e.dismissPopoverObj(s.pop) // leaves an open completion list in place
	}
	s.help = nil
	s.sig = 0
}

// sigArrow is a small clickable icon used for the overload control. It
// implements Tappable but deliberately not Focusable: tapping it cycles
// overloads without taking keyboard focus from the Entry — the Entry sits under
// the popover, so Fyne keeps it focused and the card stays open.
type sigArrow struct {
	widget.BaseWidget
	icon  *widget.Icon
	onTap func()
}

func newSigArrow(res bellui.Resource, onTap func()) *sigArrow {
	a := &sigArrow{icon: widget.NewIcon(res), onTap: onTap}
	a.ExtendBaseWidget(a)
	return a
}

func (a *sigArrow) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(a.icon)
}

func (a *sigArrow) Tapped(_ *event.PointEvent) {
	if a.onTap != nil {
		a.onTap()
	}
}

// Cursor shows the pointer cursor over the arrow to signal it is clickable.
func (a *sigArrow) Cursor() desktop.Cursor { return desktop.PointerCursor }
