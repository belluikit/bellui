package lang

import (
	"testing"

	"github.com/jeandeaual/go-locale"
	"github.com/stretchr/testify/assert"
)

func TestSystemLocale(t *testing.T) {
	info, err := locale.GetLocale()
	if err != nil {
		// something not testable
		t.Log("Unable to run locale test because", err)
		return
	}

	if len(info) < 2 {
		info = "en_US"
	}

	loc := SystemLocale()
	assert.Equal(t, info[:2], loc.String()[:2])
}

func TestCurrentLocale(t *testing.T) {
	// With an active override, CurrentLocale reflects it (unlike SystemLocale).
	SetLocaleForTests(t, "sr-RS")
	assert.Equal(t, "sr-RS", CurrentLocale().String())
	assert.True(t, IsRTL() == IsRTLLocale(CurrentLocale()),
		"CurrentLocale should agree with the source IsRTL uses")

	SetLocaleForTests(t, "ar")
	assert.Equal(t, "ar", CurrentLocale().String())
}
