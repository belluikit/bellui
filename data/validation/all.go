package validation

import (
	"github.com/belluikit/bellui"
)

// NewAllStrings creates a validator that requires all of the passed string validators to pass.
// In short, it combines multiple string validators into one.
//
// Since: 1.0
func NewAllStrings(validators ...bellui.StringValidator) bellui.StringValidator {
	return func(text string) error {
		for _, validator := range validators {
			if err := validator(text); err != nil {
				return err
			}
		}
		return nil
	}
}
