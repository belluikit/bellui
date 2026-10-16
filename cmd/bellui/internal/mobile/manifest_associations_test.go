package mobile

import (
	"bytes"
	"strings"
	"testing"

	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
	"github.com/stretchr/testify/assert"
)

func TestAndroidManifestFileHandlers(t *testing.T) {
	buf := &bytes.Buffer{}
	err := templates.ManifestAndroid.Execute(buf, manifestTmplData{
		JavaPkgPath: "com.example.testing",
		Name:        "Testing",
		LibName:     "testing",
		URLSchemes:  []string{"testing", "testing-alt"},
		MimeTypes:   []string{"text/plain"},
	})
	assert.NoError(t, err)
	out := buf.String()

	// one VIEW intent-filter per scheme
	assert.Equal(t, 2, strings.Count(out, `android:scheme="testing"`)+strings.Count(out, `android:scheme="testing-alt"`))
	assert.Contains(t, out, `<data android:scheme="testing" />`)
	assert.Contains(t, out, `<data android:scheme="testing-alt" />`)
	assert.Contains(t, out, `<data android:mimeType="text/plain" />`)
	assert.Contains(t, out, "android.intent.action.VIEW")
	assert.Contains(t, out, "android.intent.category.BROWSABLE")
}

func TestAndroidManifestNoHandlers(t *testing.T) {
	buf := &bytes.Buffer{}
	err := templates.ManifestAndroid.Execute(buf, manifestTmplData{
		JavaPkgPath: "com.example.testing",
		Name:        "Testing",
		LibName:     "testing",
	})
	assert.NoError(t, err)
	out := buf.String()

	// only the launcher intent-filter is present
	assert.False(t, strings.Contains(out, "android.intent.action.VIEW"))
	assert.Equal(t, 1, strings.Count(out, "<intent-filter>"))
}

func TestIOSEntitlementsAssociatedDomains(t *testing.T) {
	buf := &bytes.Buffer{}
	err := entitlementsTmpl.Execute(buf, []string{"example.com", "links.example.com?mode=developer"})
	assert.NoError(t, err)
	out := buf.String()

	// Universal Links (pre-rebrand tracker #622) need a com.apple.developer.associated-domains
	// entitlement with an applinks:<domain> entry per declared domain.
	assert.Contains(t, out, "<key>com.apple.developer.associated-domains</key>")
	assert.Contains(t, out, "<string>applinks:example.com</string>")
	assert.Contains(t, out, "<string>applinks:links.example.com?mode=developer</string>")
}

func TestIOSPbxprojEntitlementsWiring(t *testing.T) {
	// CODE_SIGN_ENTITLEMENTS is emitted only when an entitlements file exists,
	// so apps without associated domains don't reference a missing file.
	with := &bytes.Buffer{}
	assert.NoError(t, projPbxprojTmpl.Execute(with, projPbxprojTmplData{BundleID: "com.example.t", Type: "Auto", Entitlements: true}))
	assert.Contains(t, with.String(), "CODE_SIGN_ENTITLEMENTS = main/main.entitlements;")

	without := &bytes.Buffer{}
	assert.NoError(t, projPbxprojTmpl.Execute(without, projPbxprojTmplData{BundleID: "com.example.t", Type: "Auto"}))
	assert.NotContains(t, without.String(), "CODE_SIGN_ENTITLEMENTS")
}

func TestMimeTypesToUTIs(t *testing.T) {
	// known types map to their system UTI; unknown fall back by top-level type;
	// results are deduplicated in order.
	got := mimeTypesToUTIs([]string{"text/plain", "application/pdf", "image/png", "text/x-custom", "application/x-thing", "text/plain"})
	assert.Equal(t, []string{"public.plain-text", "com.adobe.pdf", "public.png", "public.text", "public.data"}, got)
	assert.Nil(t, mimeTypesToUTIs(nil))
}

func TestIOSInfoPlistDocumentTypes(t *testing.T) {
	buf := &bytes.Buffer{}
	mimes := []string{"text/plain"}
	err := infoplistTmpl.Execute(buf, infoplistTmplData{
		BundleID:     "com.example.testing",
		Name:         "Testing",
		Version:      "1.0.0",
		Build:        1,
		MimeTypes:    mimes,
		ContentTypes: mimeTypesToUTIs(mimes),
	})
	assert.NoError(t, err)
	out := buf.String()

	// iOS 14+ routes file-open targets by UTI (LSItemContentTypes), so the app
	// only surfaces as a handler when the UTI is emitted (pre-rebrand tracker #622 follow-up); the
	// legacy MIME list stays for back-compat.
	assert.Contains(t, out, "<key>LSItemContentTypes</key>")
	assert.Contains(t, out, "<string>public.plain-text</string>")
	assert.Contains(t, out, "<key>CFBundleDocumentTypes</key>")
	assert.Contains(t, out, "<string>text/plain</string>")
}

func TestIOSInfoPlistNoDocumentTypes(t *testing.T) {
	buf := &bytes.Buffer{}
	err := infoplistTmpl.Execute(buf, infoplistTmplData{BundleID: "com.example.testing", Name: "Testing", Version: "1.0.0", Build: 1})
	assert.NoError(t, err)
	out := buf.String()
	assert.NotContains(t, out, "CFBundleDocumentTypes")
	assert.NotContains(t, out, "LSItemContentTypes")
}
