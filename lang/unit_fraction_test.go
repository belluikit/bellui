package lang

import (
	"bytes"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A fractional magnitude picks its CLDR plural category without error. #339
// found FormatDistance(2500) logged "invalid plural count 2.5": go-i18n takes
// a float only as a string, so the float64 operand failed and the English
// fallback pattern was used, which is right in English and wrong where a
// fraction inflects the label.
func TestFormatUnit_FractionalPluralCount(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	SetLocaleForTests(t, "en-AU") // metric
	assert.Equal(t, "2.5 km", FormatDistance(2500))

	SetLocaleForTests(t, "pl") // Polish fractions take "other": mili
	assert.Equal(t, "2,5 mili", FormatUnit(2.5, UnitMile))

	assert.NotContains(t, buf.String(), "plural count")
	assert.NotContains(t, buf.String(), "Unit format failure")
}
