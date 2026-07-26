package validation_test

import (
	"testing"

	"github.com/belluikit/bellui/data/validation"

	"github.com/stretchr/testify/assert"
)

func TestValidator_Regexp(t *testing.T) {
	r := validation.NewRegexp(``, "Input is not valid")

	assert.NoError(t, r("nothing")) // Nothing to validate with, same as no validator.

	r = validation.NewRegexp(`^\d{2}-\w{4}$`, "Input is not valid")

	assert.NoError(t, r("10-bell"))
	assert.EqualError(t, r("100-belluidesk"), "Input is not valid")
}
