package lang

import (
	"golang.org/x/text/language"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/uidir"
)

// rtlScripts is the set of writing-system scripts (ISO 15924 codes) that are
// written right-to-left. It is keyed by the four-letter script code as returned
// by golang.org/x/text/language.Tag.Script().
var rtlScripts = map[string]struct{}{
	"Arab": {}, // Arabic
	"Hebr": {}, // Hebrew
	"Syrc": {}, // Syriac
	"Thaa": {}, // Thaana (Dhivehi)
	"Nkoo": {}, // N'Ko
	"Samr": {}, // Samaritan
	"Mand": {}, // Mandaic
	"Mend": {}, // Mende Kikakui
	"Adlm": {}, // Adlam
	"Rohg": {}, // Hanifi Rohingya
	"Yezi": {}, // Yezidi
}

// IsRTL reports whether the active locale uses a right-to-left script (for
// example Arabic or Hebrew). It follows the same current-locale source as the
// other lang formatting helpers, so it honours [SetLocaleForTests] and any
// app-level locale override. It is the locale source used for
// [github.com/belluikit/bellui.TextDirectionAuto].
//
// Since: 1.0
func IsRTL() bool {
	// currentTag is only populated once a locale is initialised — lazily by the
	// formatting helpers, or eagerly by an override. Before that it is the zero
	// (undefined) tag, which would mis-report an RTL system locale as LTR. Fall
	// back to the system locale on that cold-start branch, mirroring
	// [CurrentLocale]; the common (initialised) path stays a single tag read so
	// the layout, alignment and shaping hot paths are unaffected.
	tag := getCurrentTag()
	if tag == (language.Tag{}) {
		return IsRTLLocale(SystemLocale())
	}
	return isRTLTag(tag)
}

// IsRTLLocale reports whether the given locale uses a right-to-left script.
// The script is taken from the locale tag when present, otherwise inferred from
// the language (for example "ar" implies the Arabic script). Unknown or
// unparseable locales are treated as left-to-right.
//
// Since: 1.0
func IsRTLLocale(l bellui.Locale) bool {
	tag, err := language.Parse(l.String())
	if err != nil {
		return false
	}
	return isRTLTag(tag)
}

// IsRTLDirection reports whether the ambient user-interface direction is
// right-to-left. It resolves the process-wide direction setting
// ([github.com/belluikit/bellui.CurrentLayoutDirection]): an explicit
// LTR/RTL override wins, while the default [github.com/belluikit/bellui.DirectionAuto]
// defers to the active locale via [IsRTL]. Layout mirroring, direction-aware
// text alignment and the default text base direction all consult this; it
// returns false (left-to-right) by default, keeping the non-RTL path unchanged.
//
// An ambient per-subtree direction takes precedence over the global setting: a
// container.DirectionScope (or a direction-scoped window) installs one while it
// lays out and paints its subtree, so a left-to-right dialog can flow naturally
// inside a right-to-left application. When no scope is active the resolution is
// unchanged, so the common path stays a single global read.
//
// Since: 1.0
func IsRTLDirection() bool {
	if d, ok := uidir.Current(); ok {
		return d == bellui.DirectionRTL
	}
	switch bellui.CurrentLayoutDirection() {
	case bellui.DirectionLTR:
		return false
	case bellui.DirectionRTL:
		return true
	default:
		return IsRTL()
	}
}

// ResolveAlign maps a logical text alignment to the concrete alignment for the
// active layout direction. Under a right-to-left layout (see [IsRTLDirection])
// [github.com/belluikit/bellui.TextAlignLeading] and
// [github.com/belluikit/bellui.TextAlignTrailing] are swapped so that leading
// resolves to the right edge and trailing to the left;
// [github.com/belluikit/bellui.TextAlignCenter] is unaffected. Under the
// default left-to-right layout the alignment is returned unchanged.
//
// Since: 1.0
func ResolveAlign(a bellui.TextAlign) bellui.TextAlign {
	if !IsRTLDirection() {
		return a
	}
	switch a {
	case bellui.TextAlignLeading:
		return bellui.TextAlignTrailing
	case bellui.TextAlignTrailing:
		return bellui.TextAlignLeading
	default:
		return a
	}
}

// isRTLTag reports whether the script of the given tag is right-to-left. The
// script is read from the tag when explicit, otherwise inferred from the
// language. Tags with no determinable script are treated as left-to-right.
func isRTLTag(tag language.Tag) bool {
	script, conf := tag.Script()
	if conf == language.No {
		return false
	}

	_, rtl := rtlScripts[script.String()]
	return rtl
}
