package mobile

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"
)

const distName = "Apple Distribution: Jane Doe (ABCDE12345)"

// testCert makes a self-signed certificate with the given common name and
// returns its DER and upper-case SHA-1.
func testCert(t *testing.T, cn string, serial int64) ([]byte, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: cn, OrganizationalUnit: []string{"ABCDE12345"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha1.Sum(der)
	return der, strings.ToUpper(hex.EncodeToString(sum[:]))
}

// profilePlist is a decoded provisioning profile allowing the given
// certificates, with a nested Entitlements dictionary that must be skipped.
func profilePlist(name, uuid string, certs ...[]byte) []byte {
	var data strings.Builder
	for _, der := range certs {
		fmt.Fprintf(&data, "\t\t<data>%s</data>\n", base64.StdEncoding.EncodeToString(der))
	}
	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>AppIDName</key>
	<string>Example</string>
	<key>DeveloperCertificates</key>
	<array>
%s	</array>
	<key>Entitlements</key>
	<dict>
		<key>Name</key>
		<string>not the profile's name</string>
		<key>application-identifier</key>
		<string>ABCDE12345.com.example.app</string>
	</dict>
	<key>Name</key>
	<string>%s</string>
	<key>UUID</key>
	<string>%s</string>
</dict>
</plist>
`, data.String(), name, uuid))
}

// fakeSigning replaces the keychain and the installed profiles for a test.
func fakeSigning(t *testing.T, ids []signingIdentity, profiles ...profileInfo) {
	t.Helper()
	oldIDs, oldFind := listSigningIdentities, findProfile
	t.Cleanup(func() { listSigningIdentities, findProfile = oldIDs, oldFind })
	listSigningIdentities = func() ([]signingIdentity, error) { return ids, nil }
	findProfile = func(nameOrUUID string) (profileInfo, bool, error) {
		for _, p := range profiles {
			if p.Name == nameOrUUID || strings.EqualFold(p.UUID, nameOrUUID) {
				return p, true, nil
			}
		}
		return profileInfo{}, false, nil
	}
}

func TestParseProfile(t *testing.T) {
	a, shaA := testCert(t, distName, 1)
	b, shaB := testCert(t, distName, 2)
	p, err := parseProfile(profilePlist("Example App Store", "2D202DCF-FDCF-4006-9838-6255772C4439", a, b))
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Example App Store" || p.UUID != "2D202DCF-FDCF-4006-9838-6255772C4439" {
		t.Errorf("name %q uuid %q, want the top-level ones (not Entitlements' Name)", p.Name, p.UUID)
	}
	if len(p.CertSHA1s) != 2 || p.CertSHA1s[0] != shaA || p.CertSHA1s[1] != shaB {
		t.Errorf("CertSHA1s = %v, want [%s %s]", p.CertSHA1s, shaA, shaB)
	}
}

func TestParseSigningIdentities(t *testing.T) {
	out := `  1) 720455139CA18927B1FD4AD4BE205E5B6DB77E6B "Developer ID Application: Jane Doe (ABCDE12345)"
  2) e4d3966dc85a49032c2eb1bc8fe46268d4d2facf "` + distName + `"
     2 valid identities found
`
	ids := parseSigningIdentities([]byte(out))
	want := []signingIdentity{
		{"720455139CA18927B1FD4AD4BE205E5B6DB77E6B", "Developer ID Application: Jane Doe (ABCDE12345)"},
		{"E4D3966DC85A49032C2EB1BC8FE46268D4D2FACF", distName},
	}
	if fmt.Sprint(ids) != fmt.Sprint(want) {
		t.Errorf("identities = %v, want %v", ids, want)
	}
}

// Two distribution identities share a name, as Apple allows. The profile
// allows only one of them, and that one is used (#1249).
func TestResolveIOSIdentity_ProfilePicksAmongSameNamedIdentities(t *testing.T) {
	_, older := testCert(t, distName, 1)
	allowedDER, allowed := testCert(t, distName, 2)
	p, err := parseProfile(profilePlist("Example App Store", "UUID-1", allowedDER))
	if err != nil {
		t.Fatal(err)
	}
	fakeSigning(t, []signingIdentity{{older, distName}, {allowed, distName}}, p)

	for _, profile := range []string{"Example App Store", "uuid-1"} {
		got, err := ResolveIOSIdentity("Apple Distribution", profile)
		if err != nil {
			t.Fatalf("profile %q: %v", profile, err)
		}
		if got != allowed {
			t.Errorf("profile %q: identity %s, want the one the profile allows, %s (not %s)", profile, got, allowed, older)
		}
	}
}

func TestResolveIOSIdentity_SHA1PassesThrough(t *testing.T) {
	fakeSigning(t, nil)
	const sha = "E4D3966DC85A49032C2EB1BC8FE46268D4D2FACF"
	if got, err := ResolveIOSIdentity(sha, "Example App Store"); err != nil || got != sha {
		t.Errorf("ResolveIOSIdentity(sha1) = %q, %v; want the SHA-1 unchanged", got, err)
	}
}

// A profile that isn't installed where Xcode keeps profiles leaves the name as
// it was, so signing behaves as it did before.
func TestResolveIOSIdentity_ProfileNotInstalled(t *testing.T) {
	fakeSigning(t, []signingIdentity{{"E4D3966DC85A49032C2EB1BC8FE46268D4D2FACF", distName}})
	if got, err := ResolveIOSIdentity("Apple Distribution", "Somewhere Else"); err != nil || got != "Apple Distribution" {
		t.Errorf("ResolveIOSIdentity = %q, %v; want the name unchanged", got, err)
	}
}

func TestResolveIOSIdentity_Errors(t *testing.T) {
	aDER, a := testCert(t, distName, 1)
	bDER, b := testCert(t, distName, 2)
	_, other := testCert(t, distName, 3)
	onlyA, _ := parseProfile(profilePlist("Only A", "U1", aDER))
	both, _ := parseProfile(profilePlist("Both", "U2", aDER, bDER))

	t.Run("no identity the profile allows", func(t *testing.T) {
		fakeSigning(t, []signingIdentity{{other, distName}}, onlyA)
		_, err := ResolveIOSIdentity("Apple Distribution", "Only A")
		if err == nil || !strings.Contains(err.Error(), a) {
			t.Errorf("err = %v, want one naming the certificate the profile allows (%s)", err, a)
		}
	})
	t.Run("two identities the profile allows", func(t *testing.T) {
		fakeSigning(t, []signingIdentity{{a, distName}, {b, distName}}, both)
		_, err := ResolveIOSIdentity("Apple Distribution", "Both")
		if err == nil || !strings.Contains(err.Error(), a) || !strings.Contains(err.Error(), b) {
			t.Errorf("err = %v, want one listing both SHA-1s to choose from", err)
		}
	})
	t.Run("the name filters the identities", func(t *testing.T) {
		fakeSigning(t, []signingIdentity{{a, "Apple Development: Jane Doe (ABCDE12345)"}, {b, distName}}, both)
		got, err := ResolveIOSIdentity("Apple Distribution", "Both")
		if err != nil || got != b {
			t.Errorf("ResolveIOSIdentity = %q, %v; want the distribution identity %s", got, err, b)
		}
	})
}

func TestCertPEMWithSHA1(t *testing.T) {
	aDER, a := testCert(t, distName, 1)
	bDER, b := testCert(t, distName, 2)
	block := func(der []byte, sha string) string {
		return "SHA-256 hash: 00\nSHA-1 hash: " + sha + "\n" +
			string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	}
	out := []byte(block(aDER, a) + block(bDER, b))

	got, err := certPEMWithSHA1(out, strings.ToLower(b))
	if err != nil {
		t.Fatal(err)
	}
	p, _ := pem.Decode(got)
	if p == nil || string(p.Bytes) != string(bDER) {
		t.Error("returned the wrong certificate's PEM")
	}
	if _, err := certPEMWithSHA1(out, strings.Repeat("0", 40)); err == nil {
		t.Error("an unknown SHA-1 should be an error")
	}
}
