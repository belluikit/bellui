package lang

import (
	"math"

	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/belluikit/bellui"
)

// Exact SI conversion factors.
const (
	metersPerMile = 1609.344
	metersPerFoot = 0.3048
	gramsPerPound = 453.59237
)

// Unit identifies a measurement unit that FormatUnit knows how to render with a
// locale-appropriate pattern. The set is deliberately small — the handful of
// length and mass units a UI typically needs — and is extended on demand rather
// than mirroring the full CLDR unit grid.
//
// Since: 1.0
type Unit int

// Supported units. Length and mass only for now.
const (
	UnitMeter Unit = iota
	UnitKilometer
	UnitMile
	UnitFoot
	UnitGram
	UnitKilogram
	UnitPound
)

// unitPattern maps each unit to its catalog key and English fallback pattern.
// The catalog value is a CLDR plural message (one/few/many/other, keyed by count)
// whose "{Value}" placeholder is the locale-formatted number; the fallback is
// the English "other" form used when the key is absent.
var unitPattern = map[Unit]struct{ key, fallback string }{
	UnitMeter:     {"unit.meter", "{Value} m"},
	UnitKilometer: {"unit.kilometer", "{Value} km"},
	UnitMile:      {"unit.mile", "{Value} mi"},
	UnitFoot:      {"unit.foot", "{Value} ft"},
	UnitGram:      {"unit.gram", "{Value} g"},
	UnitKilogram:  {"unit.kilogram", "{Value} kg"},
	UnitPound:     {"unit.pound", "{Value} lb"},
}

// FormatUnit renders value with the given unit using the current locale's unit
// pattern and number formatting (so "1234.5" carries the locale's decimal and
// group separators, and the unit label/placement comes from the catalog). The
// label is **count-inflected** per the locale's CLDR plural rules — e.g. Polish
// UnitMile is "1 mila" / "2 mile" / "5 mil" — so the value drives which plural
// form of the label is chosen. The caller chooses the unit; use
// PreferredLengthUnit to pick km vs mi from the locale first.
//
// Since: 1.0
func FormatUnit(value float64, unit Unit) string {
	num := FormatDecimal(value, -1)
	p, ok := unitPattern[unit]
	if !ok {
		return num
	}
	return localizeUnit(p.key, p.fallback, pluralCount(value), num)
}

// localizeUnit resolves a unit plural message. It mirrors LocalizePluralKey but
// accepts a flexible count operand (int64/float64) so fractional magnitudes pick
// the right CLDR plural category — the exported wrapper is int-only.
func localizeUnit(key, fallback string, count any, value string) string {
	data := map[string]any{"Value": value}
	ret, err := localize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{ID: key, Other: fallback},
		PluralCount:    count,
		TemplateData:   data,
	})
	if err != nil {
		bellui.LogError("Unit format failure", err)
		return fallbackWithData(key, fallback, data)
	}
	return ret
}

// pluralCount reduces value to the operand the plural-rule engine keys on: an
// integer when value is whole (so "1"/"5" pick one/many cleanly), else the float
// (whose visible-fraction operands select the fractional category, usually
// "other").
func pluralCount(value float64) any {
	if value == math.Trunc(value) && math.Abs(value) < 1e15 {
		return int64(value)
	}
	return value
}

// PreferredLengthUnit reports the road-distance length unit the current locale
// prefers — UnitMile for regions on the imperial/US measurement system (US, GB,
// LR, MM), UnitKilometer everywhere else (the CLDR default). Callers pass the
// result to FormatUnit after converting the magnitude themselves.
//
// Since: 1.0
func PreferredLengthUnit() Unit {
	if _, ok := mileRegions[regionString(getCurrentTag())]; ok {
		return UnitMile
	}
	return UnitKilometer
}

// mileRegions are the regions whose measurement system uses miles for road
// distances (CLDR measurementSystem "US"/"UK"). Everywhere else is metric.
var mileRegions = map[string]bool{
	"US": true, "GB": true, "LR": true, "MM": true,
}

// FormatDistance renders a distance given in **metres** using the current
// locale's measurement system, converting the magnitude and picking a size-
// appropriate unit — so callers don't hand-roll the km-vs-mi + conversion dance
// that FormatUnit alone leaves to them (pre-rebrand tracker #861). Metric locales get metres below
// 1 km and kilometres above; imperial locales (see PreferredLengthUnit) get feet
// below 1 mile and miles above. The converted value is rounded to two decimals.
//
// Since: 1.0
func FormatDistance(meters float64) string {
	if PreferredLengthUnit() == UnitMile {
		if math.Abs(meters) < metersPerMile {
			return FormatUnit(round2(meters/metersPerFoot), UnitFoot)
		}
		return FormatUnit(round2(meters/metersPerMile), UnitMile)
	}
	if math.Abs(meters) < 1000 {
		return FormatUnit(round2(meters), UnitMeter)
	}
	return FormatUnit(round2(meters/1000), UnitKilometer)
}

// FormatMass renders a mass given in **grams** using the current locale's
// measurement system (pre-rebrand tracker #861): imperial locales (US/GB/LR/MM) get pounds; metric
// locales get grams below 1 kg and kilograms above. The converted value is
// rounded to two decimals.
//
// Since: 1.0
func FormatMass(grams float64) string {
	if _, imperial := mileRegions[regionString(getCurrentTag())]; imperial {
		return FormatUnit(round2(grams/gramsPerPound), UnitPound)
	}
	if math.Abs(grams) < 1000 {
		return FormatUnit(round2(grams), UnitGram)
	}
	return FormatUnit(round2(grams/1000), UnitKilogram)
}

// round2 rounds to two decimal places so converted magnitudes (e.g. metres→miles)
// read cleanly instead of trailing a long irrational tail.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
