package lang

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatUnit_English(t *testing.T) {
	setupLang("en")
	assert.Equal(t, "5 km", FormatUnit(5, UnitKilometer))
	assert.Equal(t, "3.5 mi", FormatUnit(3.5, UnitMile))
	assert.Equal(t, "12 ft", FormatUnit(12, UnitFoot))
	assert.Equal(t, "250 g", FormatUnit(250, UnitGram))
	assert.Equal(t, "2 kg", FormatUnit(2, UnitKilogram))
}

func TestFormatUnit_LocaleSeparators(t *testing.T) {
	// German uses a comma decimal separator; the unit label is unchanged.
	setupLang("de-DE")
	assert.Equal(t, "3,5 km", FormatUnit(3.5, UnitKilometer))
}

func TestFormatUnit_UnknownUnitReturnsNumber(t *testing.T) {
	setupLang("en")
	assert.Equal(t, "7", FormatUnit(7, Unit(999)))
}

func TestFormatDistance_Metric(t *testing.T) {
	setupLang("de-DE")
	assert.Equal(t, "300 m", FormatDistance(300))
	assert.Equal(t, "5 km", FormatDistance(5000))
	assert.Equal(t, "1,5 km", FormatDistance(1500)) // comma decimal + km switch at 1000 m
}

func TestFormatDistance_Imperial(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, "100 ft", FormatDistance(30.48))  // below 1 mile -> feet
	assert.Equal(t, "1 mi", FormatDistance(1609.344)) // exactly 1 mile
	assert.Equal(t, "2 mi", FormatDistance(2*1609.344))
}

func TestFormatMass_Metric(t *testing.T) {
	setupLang("de-DE") // regular space; fr uses a narrow NBSP (see LocaleCoverage)
	assert.Equal(t, "500 g", FormatMass(500))
	assert.Equal(t, "2 kg", FormatMass(2000))
}

func TestFormatMass_Imperial(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, "1 lb", FormatMass(453.59237))
	assert.Equal(t, "2 lb", FormatMass(2*453.59237))
}

// TestFormatUnit_PluralInflection is the pre-rebrand tracker #862 payload: the unit label is chosen
// by the value's CLDR plural category, not a fixed "other" form.
func TestFormatUnit_PluralInflection(t *testing.T) {
	setupLang("pl") // Polish: one/few/many/other = mila/mile/mil/mili
	assert.Equal(t, "1 mila", FormatUnit(1, UnitMile))
	assert.Equal(t, "2 mile", FormatUnit(2, UnitMile))
	assert.Equal(t, "5 mil", FormatUnit(5, UnitMile))
	assert.Equal(t, "22 mile", FormatUnit(22, UnitMile)) // 22 is "few" in Polish
	assert.Equal(t, "1 funt", FormatUnit(1, UnitPound))
	assert.Equal(t, "5 funtów", FormatUnit(5, UnitPound))

	setupLang("uk") // Ukrainian: миля/милі/миль
	assert.Equal(t, "1 миля", FormatUnit(1, UnitMile))
	assert.Equal(t, "2 милі", FormatUnit(2, UnitMile))
	assert.Equal(t, "5 миль", FormatUnit(5, UnitMile))
}

// TestFormatUnit_PluralInvariant covers units/locales whose label does not
// inflect: the plural machinery must still resolve cleanly even when CLDR only
// supplies the "other" form for a count that would otherwise select few/many.
func TestFormatUnit_PluralInvariant(t *testing.T) {
	setupLang("pl") // pl kilometer has only "other" in CLDR
	assert.Equal(t, "5 km", FormatUnit(5, UnitKilometer))
	assert.Equal(t, "1 km", FormatUnit(1, UnitKilometer))

	setupLang("en")
	assert.Equal(t, "1 km", FormatUnit(1, UnitKilometer))
	assert.Equal(t, "5 km", FormatUnit(5, UnitKilometer))
}

func TestPreferredLengthUnit(t *testing.T) {
	setupLang("en-US")
	assert.Equal(t, UnitMile, PreferredLengthUnit())

	setupLang("en-GB")
	assert.Equal(t, UnitMile, PreferredLengthUnit())

	setupLang("de-DE")
	assert.Equal(t, UnitKilometer, PreferredLengthUnit())

	setupLang("fr-FR")
	assert.Equal(t, UnitKilometer, PreferredLengthUnit())
}
