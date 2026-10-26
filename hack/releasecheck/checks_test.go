package main

import (
	"slices"
	"strings"
	"testing"
)

// Every check can be shown and acted on: a unique ID, a known area and
// platforms, somewhere to go, steps and an expected result.
func TestChecksAreComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range checks {
		if c.ID == "" || seen[c.ID] {
			t.Errorf("check %q: missing or duplicate ID", c.ID)
		}
		seen[c.ID] = true
		if _, ok := areaTitle[c.Area]; !ok {
			t.Errorf("%s: area %q has no title", c.ID, c.Area)
		}
		for _, p := range c.Platforms {
			if !slices.Contains(platforms, p) {
				t.Errorf("%s: unknown platform %q", c.ID, p)
			}
		}
		for p := range c.Per {
			if !slices.Contains(platforms, p) {
				t.Errorf("%s: note for unknown platform %q", c.ID, p)
			}
		}
		if c.Where == "" || len(c.Steps) == 0 || len(c.Expect) == 0 {
			t.Errorf("%s: needs Where, Steps and Expect", c.ID)
		}
		if !strings.HasPrefix(c.ID, map[string]string{
			"smoke": "SM-", "touch": "TK-", "i18n": "I18N-", "rtl": "RTL-", "a11y": "A11Y-", "charts": "CH-",
			"tables": "TB-", "dag": "DG-", "media": "MD-", "cli": "CLI-", "edge": "ED-",
		}[c.Area]) {
			t.Errorf("%s: ID prefix does not match area %q", c.ID, c.Area)
		}
	}
}

// Every platform gets checks from each of the areas the release asked for:
// i18n, accessibility and right-to-left.
func TestEveryPlatformCoversI18nA11yAndRTL(t *testing.T) {
	for _, p := range platforms {
		for _, area := range []string{"i18n", "a11y", "rtl", "edge"} {
			if len(filter(checks, p, area)) == 0 {
				t.Errorf("%s has no %s check", p, area)
			}
		}
	}
}
