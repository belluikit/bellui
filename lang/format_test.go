package lang

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWeekStartForRegion(t *testing.T) {
	assert.Equal(t, time.Sunday, weekStartForRegion("US"))
	assert.Equal(t, time.Sunday, weekStartForRegion("CA"))
	assert.Equal(t, time.Sunday, weekStartForRegion("JP"))
	assert.Equal(t, time.Sunday, weekStartForRegion("AU"))
	assert.Equal(t, time.Sunday, weekStartForRegion("IL"))
	assert.Equal(t, time.Saturday, weekStartForRegion("AF"))
	assert.Equal(t, time.Saturday, weekStartForRegion("IR"))
	assert.Equal(t, time.Saturday, weekStartForRegion("EG"))
	assert.Equal(t, time.Monday, weekStartForRegion("DE"))
	assert.Equal(t, time.Monday, weekStartForRegion("FR"))
	assert.Equal(t, time.Monday, weekStartForRegion("GB"))
	assert.Equal(t, time.Monday, weekStartForRegion("ZZ")) // unknown
}

func TestWeekStartDay(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, time.Sunday, WeekStartDay())

	setupLang("de-DE")
	assert.Equal(t, time.Monday, WeekStartDay())

	setupLang("fa-IR")
	assert.Equal(t, time.Saturday, WeekStartDay())
}

func TestDayHeaders_English(t *testing.T) {
	setupLang("en-US")
	h := DayHeaders()
	assert.Len(t, h, 7)
	assert.Equal(t, "S", h[0]) // Sunday first
	assert.Equal(t, "M", h[1])
	assert.Equal(t, "S", h[6]) // Saturday last
}

func TestDayHeaders_German(t *testing.T) {
	setupLang("de-DE")
	h := DayHeaders()
	assert.Len(t, h, 7)
	assert.Equal(t, "M", h[0]) // Monday first
	assert.Equal(t, "S", h[6]) // Sunday last
}

func TestWeekdayOffset(t *testing.T) {
	// 2025-06-01 is a Sunday.
	first := time.Date(2025, time.June, 1, 0, 0, 0, 0, time.UTC)

	setupLang("en-US") // week starts Sunday
	assert.Equal(t, 0, WeekdayOffset(first), "Sunday is first day in US locale")

	setupLang("de-DE") // week starts Monday
	assert.Equal(t, 6, WeekdayOffset(first), "Sunday is 6 days after Monday")
}

func TestWeekdayOffset_Wednesday(t *testing.T) {
	// 2025-01-01 is a Wednesday.
	first := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)

	setupLang("en-US") // Sunday start
	assert.Equal(t, 3, WeekdayOffset(first))

	setupLang("de-DE") // Monday start
	assert.Equal(t, 2, WeekdayOffset(first))
}

func TestFormatMonthYear_English(t *testing.T) {
	setupLang("en")
	dt := time.Date(2025, time.March, 15, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, "March 2025", FormatMonthYear(dt))
}

func TestFormatMonthYear_German(t *testing.T) {
	setupLang("de")
	dt := time.Date(2025, time.December, 1, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, "Dezember 2025", FormatMonthYear(dt))
}

func TestDecimalSeparator(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, ".", DecimalSeparator())

	setupLang("de-DE")
	assert.Equal(t, ",", DecimalSeparator())
}

func TestGroupSeparator(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, ",", GroupSeparator())

	setupLang("de-DE")
	assert.Equal(t, ".", GroupSeparator())
}

func TestFormatDecimal(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, "1,234.50", FormatDecimal(1234.5, 2))

	setupLang("de-DE")
	assert.Equal(t, "1.234,50", FormatDecimal(1234.5, 2))
}

func TestFormatDecimal_NegativeScale(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, "42", FormatDecimal(42.0, -1))
	assert.Equal(t, "3.14", FormatDecimal(3.14, -1))
}

func TestParseLocaleFloat(t *testing.T) {
	setupLang("en-US")
	v, err := ParseLocaleFloat("1,234.5")
	assert.NoError(t, err)
	assert.InDelta(t, 1234.5, v, 0.001)

	setupLang("de-DE")
	v, err = ParseLocaleFloat("1.234,5")
	assert.NoError(t, err)
	assert.InDelta(t, 1234.5, v, 0.001)
}

func TestParseLocaleFloat_Simple(t *testing.T) {
	setupLang("en-US")
	v, err := ParseLocaleFloat("3.14")
	assert.NoError(t, err)
	assert.InDelta(t, 3.14, v, 0.001)

	setupLang("de-DE")
	v, err = ParseLocaleFloat("3,14")
	assert.NoError(t, err)
	assert.InDelta(t, 3.14, v, 0.001)
}

func TestParseLocaleFloat_Negative(t *testing.T) {
	setupLang("en-US")
	v, err := ParseLocaleFloat("-42.5")
	assert.NoError(t, err)
	assert.InDelta(t, -42.5, v, 0.001)
}

func TestParseLocaleFloat_Invalid(t *testing.T) {
	setupLang("en-US")
	_, err := ParseLocaleFloat("abc")
	assert.Error(t, err)
}

func TestFormatParseRoundTrip(t *testing.T) {
	for _, lang := range []string{"en-US", "de-DE", "fr-FR"} {
		t.Run(lang, func(t *testing.T) {
			setupLang(lang)
			original := 12345.678
			formatted := FormatDecimal(original, 3)
			parsed, err := ParseLocaleFloat(formatted)
			assert.NoError(t, err)
			assert.InDelta(t, original, parsed, 0.001)
		})
	}
}

func TestParseLocaleFloat_EdgeCases(t *testing.T) {
	setupLang("en-US")

	v, err := ParseLocaleFloat("0")
	assert.NoError(t, err)
	assert.Equal(t, 0.0, v)

	v, err = ParseLocaleFloat("-0")
	assert.NoError(t, err)
	assert.Equal(t, 0.0, math.Abs(v))
}

// ---------------------------------------------------------------------------
// Date formatting
// ---------------------------------------------------------------------------

func TestDatePattern(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, "01/02/2006", DatePattern())

	setupLang("de-DE")
	assert.Equal(t, "02.01.2006", DatePattern())

	setupLang("ja-JP")
	assert.Equal(t, "2006/01/02", DatePattern())

	setupLang("sv-SE")
	assert.Equal(t, "2006-01-02", DatePattern()) // ISO default
}

func TestFormatDate(t *testing.T) {
	dt := time.Date(2025, time.March, 5, 0, 0, 0, 0, time.UTC)

	setupLang("en-US")
	assert.Equal(t, "03/05/2025", FormatDate(dt))

	setupLang("de-DE")
	assert.Equal(t, "05.03.2025", FormatDate(dt))

	setupLang("ja-JP")
	assert.Equal(t, "2025/03/05", FormatDate(dt))
}

func TestParseDate(t *testing.T) {
	setupLang("en-US")
	dt, err := ParseDate("03/05/2025")
	assert.NoError(t, err)
	assert.Equal(t, 2025, dt.Year())
	assert.Equal(t, time.March, dt.Month())
	assert.Equal(t, 5, dt.Day())

	setupLang("de-DE")
	dt, err = ParseDate("05.03.2025")
	assert.NoError(t, err)
	assert.Equal(t, 2025, dt.Year())
	assert.Equal(t, time.March, dt.Month())
	assert.Equal(t, 5, dt.Day())
}

func TestParseDate_SingleDigit(t *testing.T) {
	setupLang("en-US")
	dt, err := ParseDate("3/5/2025")
	assert.NoError(t, err)
	assert.Equal(t, time.March, dt.Month())
	assert.Equal(t, 5, dt.Day())

	setupLang("de-DE")
	dt, err = ParseDate("5.3.2025")
	assert.NoError(t, err)
	assert.Equal(t, time.March, dt.Month())
	assert.Equal(t, 5, dt.Day())
}

func TestParseDate_Invalid(t *testing.T) {
	setupLang("en-US")
	_, err := ParseDate("not-a-date")
	assert.Error(t, err)

	_, err = ParseDate("13/32/2025")
	assert.Error(t, err)
}

func TestDatePattern_PersianFallsBackToISO(t *testing.T) {
	// fa-IR has no entry in the short-date table, so it silently falls back to
	// the ISO 8601 layout rather than panicking (#291).
	setupLang("fa-IR")
	assert.Equal(t, "2006-01-02", DatePattern())

	dt := time.Date(2025, time.March, 5, 0, 0, 0, 0, time.UTC)
	formatted := FormatDate(dt)
	assert.Equal(t, "2025-03-05", formatted)

	parsed, err := ParseDate(formatted)
	assert.NoError(t, err)
	assert.Equal(t, 2025, parsed.Year())
	assert.Equal(t, time.March, parsed.Month())
	assert.Equal(t, 5, parsed.Day())
}

func TestDatePattern_ArabicRegionPattern(t *testing.T) {
	// ar-SA (an RTL locale) does have a day/month/year pattern entry; it must
	// format and round-trip correctly (#291).
	setupLang("ar-SA")
	assert.Equal(t, "02/01/2006", DatePattern())

	dt := time.Date(2025, time.March, 5, 0, 0, 0, 0, time.UTC)
	formatted := FormatDate(dt)
	assert.Equal(t, "05/03/2025", formatted)

	parsed, err := ParseDate(formatted)
	assert.NoError(t, err)
	assert.Equal(t, time.March, parsed.Month())
	assert.Equal(t, 5, parsed.Day())
}

func TestParseDateTime_Malformed(t *testing.T) {
	setupLang("en-US")
	for _, bad := range []string{"", "garbage", "not-a-datetime", "xx/yy/zzzz aa:bb"} {
		_, err := ParseDateTime(bad)
		assert.Error(t, err, "input %q must not parse", bad)
	}
}

func TestFormatParseDateRoundTrip(t *testing.T) {
	dt := time.Date(2025, time.July, 14, 0, 0, 0, 0, time.UTC)
	for _, lang := range []string{"en-US", "de-DE", "ja-JP", "en-GB"} {
		t.Run(lang, func(t *testing.T) {
			setupLang(lang)
			formatted := FormatDate(dt)
			parsed, err := ParseDate(formatted)
			assert.NoError(t, err)
			assert.Equal(t, dt.Year(), parsed.Year())
			assert.Equal(t, dt.Month(), parsed.Month())
			assert.Equal(t, dt.Day(), parsed.Day())
		})
	}
}

// ---------------------------------------------------------------------------
// Time formatting
// ---------------------------------------------------------------------------

func TestTimePattern(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, "3:04 PM", TimePattern())

	setupLang("de-DE")
	assert.Equal(t, "15:04", TimePattern())
}

func TestFormatTime(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, "2:30 PM", FormatTime(14, 30))
	assert.Equal(t, "9:05 AM", FormatTime(9, 5))

	setupLang("de-DE")
	assert.Equal(t, "14:30", FormatTime(14, 30))
	assert.Equal(t, "09:05", FormatTime(9, 5))
}

func TestDateTimePattern(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, "01/02/2006 3:04 PM", DateTimePattern())

	setupLang("de-DE")
	assert.Equal(t, "02.01.2006 15:04", DateTimePattern())
}

func TestFormatDateTime(t *testing.T) {
	dt := time.Date(2025, time.March, 5, 14, 30, 0, 0, time.UTC)

	setupLang("en-US")
	assert.Equal(t, "03/05/2025 2:30 PM", FormatDateTime(dt))

	setupLang("de-DE")
	assert.Equal(t, "05.03.2025 14:30", FormatDateTime(dt))
}

func TestParseDateTime(t *testing.T) {
	setupLang("en-US")
	dt, err := ParseDateTime("03/05/2025 2:30 PM")
	assert.NoError(t, err)
	assert.Equal(t, 2025, dt.Year())
	assert.Equal(t, time.March, dt.Month())
	assert.Equal(t, 5, dt.Day())
	assert.Equal(t, 14, dt.Hour())
	assert.Equal(t, 30, dt.Minute())

	setupLang("de-DE")
	dt, err = ParseDateTime("05.03.2025 14:30")
	assert.NoError(t, err)
	assert.Equal(t, 2025, dt.Year())
	assert.Equal(t, time.March, dt.Month())
	assert.Equal(t, 14, dt.Hour())
}

// ---------------------------------------------------------------------------
// 24-hour clock preference
// ---------------------------------------------------------------------------

func TestPrefers24HourClock(t *testing.T) {
	setupLang("en-US")
	assert.False(t, Prefers24HourClock(), "US uses 12-hour")

	setupLang("de-DE")
	assert.True(t, Prefers24HourClock(), "Germany uses 24-hour")

	setupLang("ja-JP")
	assert.True(t, Prefers24HourClock(), "Japan uses 24-hour")

	setupLang("en-AU")
	assert.False(t, Prefers24HourClock(), "Australia uses 12-hour")

	setupLang("fr-FR")
	assert.True(t, Prefers24HourClock(), "France uses 24-hour")

	setupLang("en-GB")
	assert.True(t, Prefers24HourClock(), "UK uses 24-hour")
}

// ---------------------------------------------------------------------------
// Currency formatting
// ---------------------------------------------------------------------------

func TestCurrencySymbol(t *testing.T) {
	assert.Equal(t, "$", CurrencySymbol("USD"))
	assert.Equal(t, "€", CurrencySymbol("EUR"))
	assert.Equal(t, "£", CurrencySymbol("GBP"))
	assert.Equal(t, "¥", CurrencySymbol("JPY"))
	assert.Equal(t, "XYZ", CurrencySymbol("XYZ")) // unknown → code
}

func TestDefaultCurrency(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, "USD", DefaultCurrency())

	setupLang("de-DE")
	assert.Equal(t, "EUR", DefaultCurrency())

	setupLang("ja-JP")
	assert.Equal(t, "JPY", DefaultCurrency())
}

func TestFormatCurrency_US(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, "$1,234.56", FormatCurrency(1234.56, "USD"))
}

func TestFormatCurrency_Germany(t *testing.T) {
	setupLang("de-DE")
	s := FormatCurrency(1234.56, "EUR")
	assert.Equal(t, "1.234,56\u00a0€", s) // "1.234,56 €" with NBSP
}

func TestFormatCurrency_UnknownCode(t *testing.T) {
	setupLang("en-US")
	s := FormatCurrency(42.00, "XYZ")
	assert.Equal(t, "XYZ42.00", s)
}

// ---------------------------------------------------------------------------
// Additional edge-case coverage
// ---------------------------------------------------------------------------

func TestFormatTime_Midnight(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, "12:00 AM", FormatTime(0, 0))

	setupLang("de-DE")
	assert.Equal(t, "00:00", FormatTime(0, 0))
}

func TestFormatTime_Noon(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, "12:00 PM", FormatTime(12, 0))

	setupLang("de-DE")
	assert.Equal(t, "12:00", FormatTime(12, 0))
}

func TestFormatTime_12hBoundary(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, "12:59 PM", FormatTime(12, 59))
	assert.Equal(t, "1:00 PM", FormatTime(13, 0))
	assert.Equal(t, "11:59 PM", FormatTime(23, 59))
}

func TestParseDateTime_SingleDigitDate(t *testing.T) {
	setupLang("en-US")
	dt, err := ParseDateTime("3/5/2025 2:30 PM")
	assert.NoError(t, err)
	assert.Equal(t, time.March, dt.Month())
	assert.Equal(t, 5, dt.Day())
	assert.Equal(t, 14, dt.Hour())
}

func TestFormatParseDateTimeRoundTrip(t *testing.T) {
	dt := time.Date(2025, time.July, 14, 9, 30, 0, 0, time.UTC)
	for _, loc := range []string{"en-US", "de-DE", "ja-JP", "en-GB"} {
		t.Run(loc, func(t *testing.T) {
			setupLang(loc)
			formatted := FormatDateTime(dt)
			parsed, err := ParseDateTime(formatted)
			assert.NoError(t, err)
			assert.Equal(t, dt.Year(), parsed.Year())
			assert.Equal(t, dt.Month(), parsed.Month())
			assert.Equal(t, dt.Day(), parsed.Day())
			assert.Equal(t, dt.Hour(), parsed.Hour())
			assert.Equal(t, dt.Minute(), parsed.Minute())
		})
	}
}

func TestFormatCurrency_Negative(t *testing.T) {
	setupLang("en-US")
	s := FormatCurrency(-1234.56, "USD")
	assert.Equal(t, "-$1,234.56", s)
}

func TestFormatCurrency_Zero(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, "$0.00", FormatCurrency(0, "USD"))
}

func TestFormatCurrency_Large(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, "$1,000,000.00", FormatCurrency(1000000, "USD"))
}

func TestFormatCurrency_France(t *testing.T) {
	setupLang("fr-FR")
	s := FormatCurrency(1234.56, "EUR")
	// France: symbol after number, NBSP separator.
	assert.Contains(t, s, "€")
	assert.Contains(t, s, "1")
}

func TestFormatCurrency_Sweden(t *testing.T) {
	setupLang("sv-SE")
	s := FormatCurrency(1234.56, "SEK")
	assert.Contains(t, s, "kr")
}

func TestDefaultCurrency_UnknownTag(t *testing.T) {
	setupLang("xx")
	assert.Equal(t, "USD", DefaultCurrency())
}

func TestPrefers24HourClock_UnknownRegion(t *testing.T) {
	setupLang("xx-ZZ")
	assert.True(t, Prefers24HourClock(), "unknown region defaults to 24-hour")
}

func TestPadDateComponents_MismatchedParts(t *testing.T) {
	// More parts in input than pattern — should return input unchanged.
	result := padDateComponents("1/2/3/4", "01/02/2006")
	assert.Equal(t, "1/2/3/4", result)
}

func TestPadDateComponents_NoSeparator(t *testing.T) {
	// Pattern with no recognisable separator — should return input unchanged.
	result := padDateComponents("20250305", "20060102")
	assert.Equal(t, "20250305", result)
}

func TestDatePattern_KR(t *testing.T) {
	setupLang("ko-KR")
	assert.Equal(t, "2006. 01. 02.", DatePattern())
}

func TestParseDate_KR_SingleDigit(t *testing.T) {
	setupLang("ko-KR")
	dt, err := ParseDate("2025. 3. 5.")
	assert.NoError(t, err)
	assert.Equal(t, 2025, dt.Year())
	assert.Equal(t, time.March, dt.Month())
	assert.Equal(t, 5, dt.Day())
}

func TestDatePatternHint(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, "MM/DD/YYYY", DatePatternHint())

	setupLang("de-DE")
	assert.Equal(t, "DD.MM.YYYY", DatePatternHint())

	setupLang("ja-JP")
	assert.Equal(t, "YYYY/MM/DD", DatePatternHint())
}

func TestTimePatternHint(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, "h:mm AM/PM", TimePatternHint())

	setupLang("de-DE")
	assert.Equal(t, "hh:mm", TimePatternHint())
}

func TestDateTimePatternHint(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, "MM/DD/YYYY h:mm AM/PM", DateTimePatternHint())

	setupLang("de-DE")
	assert.Equal(t, "DD.MM.YYYY hh:mm", DateTimePatternHint())
}

func TestFormatParseDate_LeapYear(t *testing.T) {
	setupLang("en-US")
	dt := time.Date(2024, time.February, 29, 0, 0, 0, 0, time.UTC)
	formatted := FormatDate(dt)
	parsed, err := ParseDate(formatted)
	assert.NoError(t, err)
	assert.Equal(t, time.February, parsed.Month())
	assert.Equal(t, 29, parsed.Day())
}

func TestFormatParseDateRoundTrip_KR(t *testing.T) {
	setupLang("ko-KR")
	dt := time.Date(2025, time.July, 14, 0, 0, 0, 0, time.UTC)
	formatted := FormatDate(dt)
	parsed, err := ParseDate(formatted)
	assert.NoError(t, err)
	assert.Equal(t, dt.Year(), parsed.Year())
	assert.Equal(t, dt.Month(), parsed.Month())
	assert.Equal(t, dt.Day(), parsed.Day())
}
