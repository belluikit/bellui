//go:build !ci && !android && !ios && !wasm && !tinygo

package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
)

func TestParseToastArgs(t *testing.T) {
	act, ok := parseToastArgs("bellui;body;7")
	assert.True(t, ok)
	assert.True(t, act.body)
	assert.Equal(t, uint64(7), act.token)

	act, ok = parseToastArgs("bellui;action;2;41")
	assert.True(t, ok)
	assert.False(t, act.body)
	assert.Equal(t, 2, act.action)
	assert.Equal(t, uint64(41), act.token)

	// Round-trips with the encoders.
	act, ok = parseToastArgs(toastBodyArgs(99))
	assert.True(t, ok)
	assert.True(t, act.body)
	assert.Equal(t, uint64(99), act.token)

	act, ok = parseToastArgs(toastActionArgs(99, 3))
	assert.True(t, ok)
	assert.Equal(t, 3, act.action)
	assert.Equal(t, uint64(99), act.token)

	// Rejections.
	for _, bad := range []string{"", "bellui", "bellui;body", "other;body;1", "bellui;body;x", "bellui;action;1", "bellui;action;x;1", "bellui;action;1;y"} {
		_, ok := parseToastArgs(bad)
		assert.False(t, ok, "expected %q to be rejected", bad)
	}
}

func TestToastActivatorCLSID_Stable(t *testing.T) {
	// Deterministic per AUMID, distinct across AUMIDs, and a valid v5/variant GUID.
	a1 := toastActivatorCLSID("io.bellui.probe")
	a2 := toastActivatorCLSID("io.bellui.probe")
	b := toastActivatorCLSID("io.other.app")
	assert.Equal(t, a1, a2, "must be deterministic for a given AUMID")
	assert.NotEqual(t, a1, b, "must differ across AUMIDs")
	assert.Equal(t, uint16(0x5000), a1.Data3&0xF000, "version nibble should be 5")
	assert.Equal(t, byte(0x80), a1.Data4[0]&0xC0, "variant bits should be RFC 4122")
}

func TestBuildToastXML(t *testing.T) {
	opts := &bellui.NotificationOptions{
		Title:   "Hello & <world>",
		Content: `Say "hi"`,
		Actions: []bellui.NotificationAction{
			{Label: "Reply"},
			{Label: "Later"},
		},
	}
	xml := buildToastXML(opts, 5, "file:///C:/tmp/icon.png")

	assert.True(t, strings.HasPrefix(xml, `<toast activationType="foreground" launch="bellui;body;5"`))
	assert.Contains(t, xml, `template="ToastGeneric"`)
	// Title/content are XML-escaped.
	assert.Contains(t, xml, `<text>Hello &amp; &lt;world&gt;</text>`)
	assert.Contains(t, xml, `<text>Say &quot;hi&quot;</text>`)
	// Icon is the appLogoOverride.
	assert.Contains(t, xml, `<image placement="appLogoOverride" src="file:///C:/tmp/icon.png"/>`)
	// Each action carries its index + token.
	assert.Contains(t, xml, `content="Reply" arguments="bellui;action;0;5"`)
	assert.Contains(t, xml, `content="Later" arguments="bellui;action;1;5"`)
	assert.True(t, strings.HasSuffix(xml, `</toast>`))
}

func TestBuildToastXML_NoIconNoActions(t *testing.T) {
	xml := buildToastXML(&bellui.NotificationOptions{Title: "T", Content: "C"}, 0, "")
	assert.NotContains(t, xml, "<image")
	assert.NotContains(t, xml, "<actions>")
	assert.Contains(t, xml, `launch="bellui;body;0"`)
}

func TestSanitizeFileName(t *testing.T) {
	assert.Equal(t, "Bellui App", sanitizeFileName(""))
	assert.Equal(t, "My-App", sanitizeFileName(`My\App`))
	assert.Equal(t, "a-b-c", sanitizeFileName("a/b:c"))
}
