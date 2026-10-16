package widget

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/lang"
)

// NumericEntry is an Entry that only accepts numeric input.
// Text is right-aligned by default. Non-numeric keystrokes are silently rejected.
//
// Since: 1.0
type NumericEntry struct {
	Entry

	// AllowFloat permits decimal point input. Default true.
	AllowFloat bool
	// AllowNegative permits minus sign input. Default true.
	AllowNegative bool

	// LocaleFormat enables locale-aware decimal and grouping separators.
	// When false (default), '.' is always the decimal separator.
	// When true, the locale's decimal separator is used for display and input.
	LocaleFormat bool

	// Min is the minimum allowed value. Nil means no lower bound.
	Min *float64
	// Max is the maximum allowed value. Nil means no upper bound.
	Max *float64

	// OnValueChanged is called when the text changes to a valid number.
	//
	// It fires from the text-change path (not from validation), so it runs only
	// on an actual edit, never speculatively from a [NumericEntry.Validate] call.
	// NumericEntry installs an internal [Entry.OnChanged] handler to drive this;
	// assigning OnChanged directly will disable OnValueChanged.
	OnValueChanged func(float64) `json:"-"`

	// OnTypedKey is an optional callback for key events before default handling.
	// Return true to consume the event and prevent default processing.
	OnTypedKey func(*event.KeyEvent) bool `json:"-"`
}

// NewNumericEntry creates a new right-aligned entry that accepts floating point numbers.
//
// Since: 1.0
func NewNumericEntry() *NumericEntry {
	e := &NumericEntry{
		AllowFloat:    true,
		AllowNegative: true,
	}
	e.Alignment = bellui.TextAlignTrailing
	e.ValidateWithBorder = true
	e.Validator = e.validate
	e.OnChanged = e.handleChanged
	e.ExtendBaseWidget(e)
	return e
}

// NewIntegerEntry creates a new right-aligned entry that accepts integers only.
//
// Since: 1.0
func NewIntegerEntry() *NumericEntry {
	e := &NumericEntry{
		AllowFloat:    false,
		AllowNegative: true,
	}
	e.Alignment = bellui.TextAlignTrailing
	e.ValidateWithBorder = true
	e.Validator = e.validate
	e.OnChanged = e.handleChanged
	e.ExtendBaseWidget(e)
	return e
}

// Value returns the current numeric value, or 0 and an error if the text is not a valid number.
func (e *NumericEntry) Value() (float64, error) {
	return e.parse(e.Text)
}

// parse converts a string to a number honouring LocaleFormat, treating empty and
// intermediate states ("", "-", the bare separator) as errors.
func (e *NumericEntry) parse(s string) (float64, error) {
	dec := e.decimalSep()
	if s == "" || s == "-" || s == dec || s == "-"+dec {
		return 0, fmt.Errorf("empty or incomplete")
	}
	if e.LocaleFormat {
		return lang.ParseLocaleFloat(s)
	}
	return strconv.ParseFloat(s, 64)
}

// handleChanged is the internal OnChanged handler that drives OnValueChanged.
// Firing here (rather than from validate) means the callback runs only on an
// actual text edit, never speculatively when Validate is called.
func (e *NumericEntry) handleChanged(s string) {
	if e.OnValueChanged == nil {
		return
	}
	if v, err := e.parse(s); err == nil {
		e.OnValueChanged(v)
	}
}

// SetValue sets the entry text to the string representation of the given value.
func (e *NumericEntry) SetValue(v float64) {
	if e.LocaleFormat {
		e.SetText(lang.FormatDecimal(v, -1))
		return
	}
	if e.AllowFloat {
		e.SetText(strconv.FormatFloat(v, 'f', -1, 64))
	} else {
		e.SetText(strconv.FormatInt(int64(v), 10))
	}
}

// SetBounds sets both Min and Max. Pass math.Inf(-1) or math.Inf(1) to clear a bound.
func (e *NumericEntry) SetBounds(min, max float64) {
	if math.IsInf(min, -1) {
		e.Min = nil
	} else {
		e.Min = &min
	}
	if math.IsInf(max, 1) {
		e.Max = nil
	} else {
		e.Max = &max
	}
	e.Validate()
}

// TypedKey forwards key events to OnTypedKey if set, then to the Entry.
func (e *NumericEntry) TypedKey(key *event.KeyEvent) {
	if e.OnTypedKey != nil && e.OnTypedKey(key) {
		return
	}
	e.Entry.TypedKey(key)
}

// TypedRune filters non-numeric characters before passing to the Entry.
func (e *NumericEntry) TypedRune(r rune) {
	if unicode.IsDigit(r) {
		e.Entry.TypedRune(r)
		return
	}
	dec := e.decimalSep()
	if decRune, ok := decimalSepRune(dec); ok && r == decRune && e.AllowFloat {
		// Only allow one decimal separator.
		if !strings.Contains(e.Text, dec) {
			e.Entry.TypedRune(r)
		}
		return
	}
	if e.LocaleFormat {
		grp := lang.GroupSeparator()
		if grp != "" && r == []rune(grp)[0] {
			e.Entry.TypedRune(r)
			return
		}
	}
	if r == '-' && e.AllowNegative {
		// Only allow minus at the beginning.
		if e.CursorColumn == 0 && !strings.Contains(e.Text, "-") {
			e.Entry.TypedRune(r)
		}
		return
	}
	// Reject everything else silently.
}

func (e *NumericEntry) decimalSep() string {
	if e.LocaleFormat {
		return lang.DecimalSeparator()
	}
	return "."
}

// decimalSepRune returns the first rune of a decimal separator string, guarding
// against an empty separator from a misconfigured locale.
func decimalSepRune(dec string) (rune, bool) {
	r := []rune(dec)
	if len(r) == 0 {
		return 0, false
	}
	return r[0], true
}

// ReplaceAccessibleSelection filters non-numeric content before applying the
// edit, so range replacements that bypass TypedRune — the accessibility text
// protocols and the mobile text-input / IME mirror (#771), which route
// select-and-replace and paste-like edits through this path — still respect the
// numeric constraint (#782). An empty replacement (a deletion) passes through;
// a replacement that filters down to nothing is dropped, matching TypedRune's
// silent rejection.
func (e *NumericEntry) ReplaceAccessibleSelection(text string) {
	if text != "" {
		text = e.filterPasteContent(text)
		if text == "" {
			return
		}
	}
	e.Entry.ReplaceAccessibleSelection(text)
}

// TypedShortcut intercepts paste to filter non-numeric content.
func (e *NumericEntry) TypedShortcut(s core.Shortcut) {
	if paste, ok := s.(*bellui.ShortcutPaste); ok {
		content := paste.Clipboard.Content()
		filtered := e.filterPasteContent(content)
		if filtered == "" {
			return
		}
		// Feed the filtered text through an in-memory clipboard rather than
		// mutating (and restoring) the system clipboard, which is racy and
		// would clobber it if the embedded paste panicked before restore.
		e.Entry.TypedShortcut(&bellui.ShortcutPaste{Clipboard: &memoryClipboard{content: filtered}})
		return
	}
	e.Entry.TypedShortcut(s)
}

// memoryClipboard is a non-system, in-memory clipboard used to deliver filtered
// paste content to the embedded Entry without touching the real clipboard.
type memoryClipboard struct {
	content string
}

func (c *memoryClipboard) Content() string     { return c.content }
func (c *memoryClipboard) SetContent(s string) { c.content = s }

func (e *NumericEntry) filterPasteContent(s string) string {
	var b strings.Builder
	dec := e.decimalSep()
	decRune, hasDecSep := decimalSepRune(dec)
	hasDec := strings.Contains(e.Text, dec)
	hasMinus := strings.Contains(e.Text, "-")

	var grpRune rune
	if e.LocaleFormat {
		grp := lang.GroupSeparator()
		if grp != "" {
			grpRune = []rune(grp)[0]
		}
	}

	for i, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
			continue
		}
		if hasDecSep && r == decRune && e.AllowFloat && !hasDec {
			hasDec = true
			b.WriteRune(r)
			continue
		}
		if grpRune != 0 && r == grpRune {
			b.WriteRune(r)
			continue
		}
		if r == '-' && e.AllowNegative && !hasMinus && i == 0 && e.CursorColumn == 0 {
			hasMinus = true
			b.WriteRune(r)
			continue
		}
	}
	return b.String()
}

func (e *NumericEntry) validate(s string) error {
	dec := e.decimalSep()
	if s == "" || s == "-" || s == dec || s == "-"+dec {
		return nil // allow intermediate states while typing
	}
	var v float64
	var err error
	if e.LocaleFormat {
		v, err = lang.ParseLocaleFloat(s)
	} else {
		v, err = strconv.ParseFloat(s, 64)
	}
	if err != nil {
		return fmt.Errorf("invalid number")
	}
	if !e.AllowFloat && s != strconv.FormatInt(int64(v), 10) {
		// Has decimal in integer-only mode (could happen via SetText).
		return fmt.Errorf("integer required")
	}
	if e.Min != nil && v < *e.Min {
		return fmt.Errorf("minimum is %g", *e.Min)
	}
	if e.Max != nil && v > *e.Max {
		return fmt.Errorf("maximum is %g", *e.Max)
	}
	return nil
}
