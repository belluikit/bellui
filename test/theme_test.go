package test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"
)

func Test_NewTheme(t *testing.T) {
	suite.Run(t, &configurableThemeTestSuite{
		constructor: NewTheme,
		name:        "Ugly Test Theme",
	})
}

func Test_Theme(t *testing.T) {
	suite.Run(t, &configurableThemeTestSuite{
		constructor: Theme,
		name:        "Default Test Theme",
	})
}

type configurableThemeTestSuite struct {
	suite.Suite
	constructor func() bellui.Theme
	name        string
}

func (s *configurableThemeTestSuite) TestAllColorsDefined() {
	AssertAllColorNamesDefined(s.T(), s.constructor(), s.name)
}

// TestAllSizesDefined checks the theme defines every size name the theme
// package knows, so a widget never lays out with a size the test theme lacks:
// that logs an error on every layout and pins goldens at 0 (#1079).
// theme.ValidateTheme walks the known names and reports each that reads 0; a
// name the theme leaves out reads 0 too, so each one reported must be defined
// as 0 on purpose.
func (s *configurableThemeTestSuite) TestAllSizesDefined() {
	t := s.T()
	th := s.constructor()
	sizes := th.(*configurableTheme).sizes
	var zero []string
	for _, issue := range theme.ValidateTheme(th) {
		if name, ok := strings.CutPrefix(issue, "zero size: "); ok {
			zero = append(zero, name)
			_, defined := sizes[bellui.ThemeSizeName(name)]
			assert.True(t, defined, "size %s not defined in theme %s", name, s.name)
		}
	}
	// shapeNone is 0 in both themes: seeing it shows the known sizes were walked.
	assert.Contains(t, zero, string(theme.SizeNameShapeNone), "theme.ValidateTheme reported no zero sizes")
}

func (s *configurableThemeTestSuite) TestUniqueColorValues() {
	t := s.T()
	th := s.constructor()
	seenByVariant := map[bellui.ThemeVariant]map[string]bellui.ThemeColorName{}
	for variantName, variant := range KnownThemeVariants() {
		seen := seenByVariant[variant]
		if seen == nil {
			seen = map[string]bellui.ThemeColorName{}
			seenByVariant[variant] = seen
		}
		for _, cn := range knownColorNames {
			c := th.Color(cn, variant)
			r, g, b, a := c.RGBA()
			key := fmt.Sprintf("%d %d %d %d", r, g, b, a)
			assert.True(t, seen[key] == "", "color value %#v for color %s variant %s already used for color %s in theme %s", c, cn, variantName, seen[key], s.name)
			seen[key] = cn
		}
	}
}
