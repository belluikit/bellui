package lang

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/currency"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/number"
)

// WeekStartDay returns the first day of the week for the current locale,
// based on CLDR region data.
//
// Since: 1.0
func WeekStartDay() time.Weekday {
	return weekStartForRegion(regionString(getCurrentTag()))
}

// DayHeaders returns 7 single-character (narrow) day abbreviations starting
// from the locale's week start day.
//
// Since: 1.0
func DayHeaders() []string {
	start := WeekStartDay()
	headers := make([]string, 7)
	for i := range headers {
		day := time.Weekday((int(start) + i) % 7)
		headers[i] = narrowDayName(day)
	}
	return headers
}

// WeekdayOffset returns how many empty cells precede day 1 in a calendar grid
// that starts on the locale's week start day.
//
// Since: 1.0
func WeekdayOffset(firstOfMonth time.Time) int {
	start := WeekStartDay()
	wd := int(firstOfMonth.Weekday())
	return (wd - int(start) + 7) % 7
}

// FormatMonthYear returns a locale-formatted "Month Year" string.
//
// Since: 1.0
func FormatMonthYear(t time.Time) string {
	month := localizedMonthName(t.Month())
	return fmt.Sprintf("%s %d", month, t.Year())
}

// DecimalSeparator returns the locale's decimal separator (e.g., "." or ",").
//
// Since: 1.0
func DecimalSeparator() string {
	sep, _ := detectSeparators()
	return sep
}

// GroupSeparator returns the locale's grouping/thousands separator (e.g., "," or "." or "\u00a0").
//
// Since: 1.0
func GroupSeparator() string {
	_, grp := detectSeparators()
	return grp
}

// FormatDecimal formats a float64 using the current locale's number formatting.
// If scale < 0, minimal fractional digits are used (trailing zeros removed).
//
// Since: 1.0
func FormatDecimal(v float64, scale int) string {
	tag := resolveTag()
	p := message.NewPrinter(tag)
	if scale < 0 {
		// Format with enough precision, then trim trailing zeros.
		s := p.Sprintf("%v", number.Decimal(v, number.MaxFractionDigits(10)))
		dec := DecimalSeparator()
		if strings.Contains(s, dec) {
			s = strings.TrimRight(s, "0")
			s = strings.TrimRight(s, dec)
		}
		return s
	}
	return p.Sprintf("%v", number.Decimal(v, number.Scale(scale)))
}

// ParseLocaleFloat parses a locale-formatted number string back to float64.
// It strips group separators and normalizes the decimal separator to '.'.
//
// Since: 1.0
func ParseLocaleFloat(s string) (float64, error) {
	dec := DecimalSeparator()
	grp := GroupSeparator()

	// Strip grouping separators.
	if grp != "" {
		s = strings.ReplaceAll(s, grp, "")
	}
	// Normalize decimal separator to '.'.
	if dec != "." {
		s = strings.ReplaceAll(s, dec, ".")
	}
	return strconv.ParseFloat(s, 64)
}

// --- internal helpers ---

func resolveTag() language.Tag {
	tag := getCurrentTag()
	if tag == language.Und {
		return language.English
	}
	return tag
}

func regionString(tag language.Tag) string {
	r, _ := tag.Region()
	return r.String()
}

func narrowDayName(d time.Weekday) string {
	switch d {
	case time.Sunday:
		return X("sunday.narrow", "S")
	case time.Monday:
		return X("monday.narrow", "M")
	case time.Tuesday:
		return X("tuesday.narrow", "T")
	case time.Wednesday:
		return X("wednesday.narrow", "W")
	case time.Thursday:
		return X("thursday.narrow", "T")
	case time.Friday:
		return X("friday.narrow", "F")
	case time.Saturday:
		return X("saturday.narrow", "S")
	}
	return "?"
}

func localizedMonthName(m time.Month) string {
	switch m {
	case time.January:
		return X("january", "January")
	case time.February:
		return X("february", "February")
	case time.March:
		return X("march", "March")
	case time.April:
		return X("april", "April")
	case time.May:
		return X("may", "May")
	case time.June:
		return X("june", "June")
	case time.July:
		return X("july", "July")
	case time.August:
		return X("august", "August")
	case time.September:
		return X("september", "September")
	case time.October:
		return X("october", "October")
	case time.November:
		return X("november", "November")
	case time.December:
		return X("december", "December")
	}
	return m.String()
}

// goLayoutToHint converts a Go time layout string to a human-readable format
// hint, e.g. "01/02/2006" → "MM/DD/YYYY", "15:04" → "HH:MM".
func goLayoutToHint(layout string) string {
	r := strings.NewReplacer(
		"2006", "YYYY",
		"01", "MM",
		"02", "DD",
		"15", "hh",
		"04", "mm",
		"3", "h",
		"PM", "AM/PM",
	)
	return r.Replace(layout)
}

// detectSeparators formats a known number to discover the locale's separators.
func detectSeparators() (decimal, group string) {
	tag := resolveTag()
	p := message.NewPrinter(tag)
	// Format 1234.5 to observe both separators.
	s := p.Sprintf("%v", number.Decimal(1234.5, number.Scale(1)))
	// The output will be something like "1,234.5" (en) or "1.234,5" (de) or "1 234,5" (fr).
	// The last non-digit rune is the decimal separator.
	// Any non-digit rune before that is the group separator.
	var lastNonDigit, prevNonDigit rune
	for _, r := range s {
		if r < '0' || r > '9' {
			prevNonDigit = lastNonDigit
			lastNonDigit = r
		}
	}
	if lastNonDigit != 0 {
		decimal = string(lastNonDigit)
	} else {
		decimal = "."
	}
	if prevNonDigit != 0 {
		group = string(prevNonDigit)
	}
	return
}

// weekStartForRegion returns the week start day for a given ISO 3166-1 region code.
// Data sourced from CLDR supplemental/weekData.
func weekStartForRegion(region string) time.Weekday {
	if d, ok := sundayStart[region]; ok {
		return d
	}
	if d, ok := saturdayStart[region]; ok {
		return d
	}
	return time.Monday // most common worldwide
}

var sundayStart = map[string]time.Weekday{
	"AG": time.Sunday, "AS": time.Sunday, "AU": time.Sunday,
	"BD": time.Sunday, "BR": time.Sunday, "BS": time.Sunday,
	"BT": time.Sunday, "BW": time.Sunday, "BZ": time.Sunday,
	"CA": time.Sunday, "CN": time.Sunday, "CO": time.Sunday,
	"DM": time.Sunday, "DO": time.Sunday, "ET": time.Sunday,
	"GT": time.Sunday, "GU": time.Sunday, "HK": time.Sunday,
	"HN": time.Sunday, "ID": time.Sunday, "IL": time.Sunday,
	"IN": time.Sunday, "JM": time.Sunday, "JP": time.Sunday,
	"KE": time.Sunday, "KH": time.Sunday, "KR": time.Sunday,
	"LA": time.Sunday, "MH": time.Sunday, "MM": time.Sunday,
	"MO": time.Sunday, "MT": time.Sunday, "MX": time.Sunday,
	"MZ": time.Sunday, "NI": time.Sunday, "NP": time.Sunday,
	"PA": time.Sunday, "PE": time.Sunday, "PH": time.Sunday,
	"PK": time.Sunday, "PR": time.Sunday, "PT": time.Sunday,
	"PY": time.Sunday, "SA": time.Sunday, "SG": time.Sunday,
	"SV": time.Sunday, "TH": time.Sunday, "TT": time.Sunday,
	"TW": time.Sunday, "UM": time.Sunday, "US": time.Sunday,
	"VE": time.Sunday, "VI": time.Sunday, "WS": time.Sunday,
	"YE": time.Sunday, "ZA": time.Sunday, "ZW": time.Sunday,
}

var saturdayStart = map[string]time.Weekday{
	"AF": time.Saturday, "BH": time.Saturday, "DJ": time.Saturday,
	"DZ": time.Saturday, "EG": time.Saturday, "IQ": time.Saturday,
	"IR": time.Saturday, "JO": time.Saturday, "KW": time.Saturday,
	"LY": time.Saturday, "OM": time.Saturday, "QA": time.Saturday,
	"SD": time.Saturday, "SY": time.Saturday,
}

// ---------------------------------------------------------------------------
// Date / time formatting
// ---------------------------------------------------------------------------

// DatePattern returns the Go time layout string for the current locale's
// short date format (e.g. "01/02/2006" for en-US, "02.01.2006" for de-DE).
//
// Since: 1.0
func DatePattern() string {
	if p, ok := shortDateFormats[regionString(getCurrentTag())]; ok {
		return p
	}
	return "2006-01-02" // ISO 8601 default
}

// DatePatternHint returns a human-readable placeholder for the locale's date
// format (e.g. "MM/DD/YYYY" for en-US, "DD.MM.YYYY" for de-DE).
//
// Since: 1.0
func DatePatternHint() string {
	return goLayoutToHint(DatePattern())
}

// FormatDate formats t using the locale's short date pattern.
//
// Since: 1.0
func FormatDate(t time.Time) string {
	return t.Format(DatePattern())
}

// ParseDate parses a locale-formatted date string.
// It is lenient with single-digit day/month values (e.g. "1/5/2025" matches "01/02/2006").
//
// Since: 1.0
func ParseDate(s string) (time.Time, error) {
	pattern := DatePattern()
	// Try strict parse first.
	if t, err := time.Parse(pattern, s); err == nil {
		return t, nil
	}
	// Flexible: pad single-digit numeric components.
	padded := padDateComponents(s, pattern)
	return time.Parse(pattern, padded)
}

// TimePattern returns the Go time layout for the locale's time-of-day format.
// Returns "3:04 PM" for 12-hour locales or "15:04" for 24-hour locales.
//
// Since: 1.0
func TimePattern() string {
	if Prefers24HourClock() {
		return "15:04"
	}
	return "3:04 PM"
}

// TimePatternHint returns a human-readable placeholder for the time format
// (e.g. "HH:MM" for 24h or "H:MM AM/PM" for 12h).
//
// Since: 1.0
func TimePatternHint() string {
	return goLayoutToHint(TimePattern())
}

// FormatTime formats a time-of-day using the locale's time pattern.
//
// Since: 1.0
func FormatTime(hour, minute int) string {
	t := time.Date(0, 1, 1, hour, minute, 0, 0, time.UTC)
	return t.Format(TimePattern())
}

// DateTimePattern returns the combined date+time layout for the locale.
//
// Since: 1.0
func DateTimePattern() string {
	return DatePattern() + " " + TimePattern()
}

// DateTimePatternHint returns a human-readable placeholder for the combined
// date+time format (e.g. "MM/DD/YYYY H:MM AM/PM" or "DD.MM.YYYY HH:MM").
//
// Since: 1.0
func DateTimePatternHint() string {
	return DatePatternHint() + " " + TimePatternHint()
}

// FormatDateTime formats t using the locale's combined date+time pattern.
//
// Since: 1.0
func FormatDateTime(t time.Time) string {
	return t.Format(DateTimePattern())
}

// ParseDateTime parses a locale-formatted date+time string.
//
// Since: 1.0
func ParseDateTime(s string) (time.Time, error) {
	pattern := DateTimePattern()
	if t, err := time.Parse(pattern, s); err == nil {
		return t, nil
	}
	// Split into date and time portions. The date pattern and time pattern are
	// joined by " ", so find the split point by counting numeric groups:
	// the date has 3 numeric groups (month, day, year), the rest is time.
	datePattern := DatePattern()
	splitIdx := findDateTimeSplit(s, datePattern)
	if splitIdx > 0 && splitIdx < len(s) {
		datePart := s[:splitIdx]
		timePart := s[splitIdx:] // includes leading space
		padded := padDateComponents(datePart, datePattern)
		return time.Parse(pattern, padded+timePart)
	}
	return time.Parse(pattern, s)
}

// findDateTimeSplit finds the index where the date portion ends and the time
// portion begins in a combined datetime string. It counts numeric groups in the
// date pattern and advances past the same number in the input string.
func findDateTimeSplit(s, datePattern string) int {
	// Count numeric groups in the date pattern.
	patGroups := 0
	inNum := false
	for _, r := range datePattern {
		isDigit := r >= '0' && r <= '9'
		if isDigit && !inNum {
			patGroups++
		}
		inNum = isDigit
	}

	// Walk the input and consume the same number of numeric groups.
	groups := 0
	inNum = false
	for i, r := range s {
		isDigit := r >= '0' && r <= '9'
		if isDigit && !inNum {
			groups++
		}
		inNum = isDigit
		if groups > patGroups {
			// Backtrack to the space before this group.
			for j := i - 1; j >= 0; j-- {
				if s[j] == ' ' {
					return j
				}
			}
			return i
		}
	}
	return -1 // couldn't find split
}

// padDateComponents pads single-digit numeric parts to two digits to match
// Go's reference-time layout expectations (e.g. "1/5/2025" → "01/05/2025").
// It extracts numeric runs from both pattern and input, pads 1-digit runs
// where the pattern expects 2 digits, then reconstructs the string preserving
// the original non-digit separators from the input.
func padDateComponents(s, pattern string) string {
	// Extract numeric runs and their positions from the input.
	type numRun struct {
		start, end int
		text       string
	}
	extractNums := func(str string) []numRun {
		var runs []numRun
		i := 0
		for i < len(str) {
			if str[i] >= '0' && str[i] <= '9' {
				j := i + 1
				for j < len(str) && str[j] >= '0' && str[j] <= '9' {
					j++
				}
				runs = append(runs, numRun{i, j, str[i:j]})
				i = j
			} else {
				i++
			}
		}
		return runs
	}

	patNums := extractNums(pattern)
	srcNums := extractNums(s)
	if len(patNums) != len(srcNums) {
		return s
	}

	// Build result by walking the input and replacing numeric runs.
	var b strings.Builder
	prev := 0
	for i, sn := range srcNums {
		b.WriteString(s[prev:sn.start])
		pn := patNums[i]
		if len(pn.text) == 2 && len(sn.text) == 1 {
			b.WriteByte('0')
		}
		b.WriteString(sn.text)
		prev = sn.end
	}
	b.WriteString(s[prev:])
	return b.String()
}

// shortDateFormats maps ISO 3166-1 region codes to Go time layout strings.
// Data derived from CLDR supplemental/dateFormats (short pattern).
var shortDateFormats = map[string]string{
	// M/D/Y order
	"US": "01/02/2006", "FM": "01/02/2006", "MH": "01/02/2006",
	"PA": "01/02/2006", "PW": "01/02/2006",
	// D/M/Y with slash
	"GB": "02/01/2006", "AU": "02/01/2006", "NZ": "02/01/2006",
	"IE": "02/01/2006", "IN": "02/01/2006", "ZA": "02/01/2006",
	"AE": "02/01/2006", "EG": "02/01/2006", "SA": "02/01/2006",
	"BR": "02/01/2006", "PT": "02/01/2006", "IT": "02/01/2006",
	"ES": "02/01/2006", "AR": "02/01/2006", "CL": "02/01/2006",
	"CO": "02/01/2006", "MX": "02/01/2006", "PE": "02/01/2006",
	"VE": "02/01/2006", "FR": "02/01/2006", "BE": "02/01/2006",
	"NL": "02-01-2006", "ID": "02/01/2006", "MY": "02/01/2006",
	"PH": "02/01/2006", "SG": "02/01/2006", "TH": "02/01/2006",
	"VN": "02/01/2006", "PK": "02/01/2006", "BD": "02/01/2006",
	"NG": "02/01/2006", "KE": "02/01/2006", "GH": "02/01/2006",
	"IL": "02/01/2006", "TR": "02.01.2006", "RU": "02.01.2006",
	"UA": "02.01.2006", "PL": "02.01.2006", "CZ": "02.01.2006",
	"SK": "02.01.2006", "RO": "02.01.2006", "BG": "02.01.2006",
	"HR": "02.01.2006", "RS": "02.01.2006", "SI": "02.01.2006",
	// D.M.Y with dot
	"DE": "02.01.2006", "AT": "02.01.2006", "CH": "02.01.2006",
	"FI": "02.01.2006", "NO": "02.01.2006",
	// D-M-Y with dash
	"DK": "02-01-2006",
	// Y/M/D order
	"JP": "2006/01/02", "CN": "2006/01/02", "TW": "2006/01/02",
	// Y.M.D with dot and trailing dot
	"KR": "2006. 01. 02.",
	// Y-M-D (ISO)
	"CA": "2006-01-02", "SE": "2006-01-02", "LT": "2006-01-02",
	"HU": "2006. 01. 02.",
}

// ---------------------------------------------------------------------------
// 24-hour / 12-hour clock preference
// ---------------------------------------------------------------------------

// Prefers24HourClock returns true if the current locale conventionally uses
// 24-hour time notation. Most of the world uses 24-hour; the map lists the
// minority of regions that prefer 12-hour clocks.
//
// Since: 1.0
func Prefers24HourClock() bool {
	return !uses12HourClock[regionString(getCurrentTag())]
}

// uses12HourClock lists regions where 12-hour (AM/PM) notation is the norm.
// Data sourced from CLDR supplemental/timeData.
var uses12HourClock = map[string]bool{
	"US": true, "CA": true, "AU": true, "NZ": true,
	"PH": true, "MY": true, "IN": true, "PK": true,
	"BD": true, "CO": true, "SV": true, "HN": true,
	"NI": true, "KR": true,
}

// ---------------------------------------------------------------------------
// Currency formatting
// ---------------------------------------------------------------------------

// FormatCurrency formats amount using the locale's number formatting and the
// appropriate currency symbol. currencyCode is an ISO 4217 code (e.g. "USD").
//
// Since: 1.0
func FormatCurrency(amount float64, currencyCode string) string {
	sym := CurrencySymbol(currencyCode)
	negative := amount < 0
	if negative {
		amount = -amount
	}
	num := FormatDecimal(amount, 2)
	var s string
	if symbolPlacedAfter(regionString(getCurrentTag())) {
		s = num + "\u00a0" + sym // non-breaking space
	} else {
		s = sym + num
	}
	if negative {
		return "-" + s
	}
	return s
}

// CurrencySymbol returns the narrow symbol for an ISO 4217 currency code.
// For example, "USD" → "$", "EUR" → "€", "GBP" → "£".
//
// Since: 1.0
func CurrencySymbol(currencyCode string) string {
	if sym, ok := currencySymbols[currencyCode]; ok {
		return sym
	}
	return currencyCode
}

// DefaultCurrency returns the ISO 4217 currency code for the current locale's
// region (e.g. "USD" for en-US, "EUR" for de-DE, "JPY" for ja-JP).
//
// Since: 1.0
func DefaultCurrency() string {
	unit, conf := currency.FromTag(resolveTag())
	if conf >= language.Low {
		return unit.String()
	}
	return "USD"
}

// currencySymbols maps ISO 4217 codes to their narrow/common symbols.
var currencySymbols = map[string]string{
	"USD": "$", "EUR": "€", "GBP": "£", "JPY": "¥", "CNY": "¥",
	"KRW": "₩", "INR": "₹", "RUB": "₽", "TRY": "₺", "BRL": "R$",
	"ZAR": "R", "CHF": "CHF", "SEK": "kr", "NOK": "kr", "DKK": "kr",
	"PLN": "zł", "CZK": "Kč", "HUF": "Ft", "RON": "lei", "BGN": "лв",
	"HRK": "kn", "RSD": "din", "UAH": "₴", "ILS": "₪", "AED": "د.إ",
	"SAR": "﷼", "EGP": "E£", "NGN": "₦", "KES": "KSh", "GHS": "GH₵",
	"AUD": "A$", "NZD": "NZ$", "CAD": "CA$", "MXN": "MX$", "ARS": "$",
	"CLP": "$", "COP": "$", "PEN": "S/", "VES": "Bs.",
	"PHP": "₱", "MYR": "RM", "IDR": "Rp", "THB": "฿", "VND": "₫",
	"SGD": "S$", "HKD": "HK$", "TWD": "NT$", "PKR": "₨", "BDT": "৳",
}

// symbolPlacedAfter returns true for regions where the currency symbol is
// conventionally placed after the number (e.g. "1.234,56 €" in Germany).
var symbolAfterRegions = map[string]bool{
	"DE": true, "AT": true, "FR": true, "BE": true, "ES": true,
	"IT": true, "PT": true, "NL": true, "FI": true, "LU": true,
	"GR": true, "SK": true, "SI": true, "LT": true, "LV": true,
	"EE": true, "HR": true, "BG": true, "RO": true, "CZ": true,
	"HU": true, "PL": true, "SE": true, "DK": true, "NO": true,
	"TR": true, "RU": true, "UA": true, "RS": true, "MT": true,
	"VN": true, "ID": true,
}

func symbolPlacedAfter(region string) bool {
	return symbolAfterRegions[region]
}
