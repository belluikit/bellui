package mobile

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// An iOS signing certificate is named after its kind and owner ("Apple
// Distribution: <name> (<team>)"), and Apple allows several of the same kind,
// so a keychain routinely holds two identities with one name. codesign refuses
// a name that matches both as ambiguous, and `security find-certificate -c`
// matches names only (#1249). The release therefore signs with a SHA-1
// fingerprint, which names one identity: the one given with -certificate, or
// the one the provisioning profile lists.

var certSHA1Pattern = regexp.MustCompile(`^[0-9A-Fa-f]{40}$`)

// IsCertSHA1 reports whether s is a certificate's SHA-1 fingerprint (40 hex
// digits), which codesign and Xcode accept as a signing identity.
func IsCertSHA1(s string) bool {
	return certSHA1Pattern.MatchString(s)
}

// signingIdentity is a code-signing identity in the keychain.
type signingIdentity struct {
	SHA1 string // upper-case hex
	Name string
}

// profileInfo is what a provisioning profile says about its signing.
type profileInfo struct {
	Name, UUID string
	// CertSHA1s are the SHA-1 fingerprints (upper-case hex) of the
	// certificates the profile allows (DeveloperCertificates).
	CertSHA1s []string
}

// listSigningIdentities and findProfile reach the keychain and the profile
// directories. Tests replace them.
var (
	listSigningIdentities = keychainSigningIdentities
	findProfile           = installedProfile
)

// ResolveIOSIdentity returns the signing identity for an iOS release: cert
// when it is already a SHA-1, otherwise the SHA-1 of the one identity in the
// keychain that the provisioning profile lists and whose name starts with cert
// (any name when cert is empty). When the profile isn't installed where Xcode
// keeps profiles, cert is returned unchanged and signing works as it did
// before. It is an error when the profile is installed but no identity, or
// more than one, matches.
func ResolveIOSIdentity(cert, profile string) (string, error) {
	if IsCertSHA1(cert) || profile == "" {
		return cert, nil
	}
	p, ok, err := findProfile(profile)
	if err != nil || !ok {
		return cert, err
	}
	ids, err := listSigningIdentities()
	if err != nil {
		return "", err
	}
	allowed := map[string]bool{}
	for _, h := range p.CertSHA1s {
		allowed[h] = true
	}
	var match []signingIdentity
	for _, id := range ids {
		if allowed[id.SHA1] && strings.HasPrefix(id.Name, cert) {
			match = append(match, id)
		}
	}
	switch len(match) {
	case 1:
		return match[0].SHA1, nil
	case 0:
		return "", fmt.Errorf("no signing identity in the keychain is one profile %q allows (%s)%s: install its certificate, or pass -certificate",
			p.Name, strings.Join(p.CertSHA1s, ", "), namedSuffix(cert))
	default:
		var list []string
		for _, id := range match {
			list = append(list, id.SHA1+" "+id.Name)
		}
		return "", fmt.Errorf("profile %q allows %d signing identities in the keychain; pass -certificate with the SHA-1 of the one to use:\n  %s",
			p.Name, len(match), strings.Join(list, "\n  "))
	}
}

func namedSuffix(cert string) string {
	if cert == "" {
		return ""
	}
	return fmt.Sprintf(" named %q", cert)
}

// keychainSigningIdentities lists the valid code-signing identities.
func keychainSigningIdentities() ([]signingIdentity, error) {
	out, err := exec.Command("security", "find-identity", "-v", "-p", "codesigning").Output()
	if err != nil {
		return nil, fmt.Errorf("listing signing identities: %w", err)
	}
	return parseSigningIdentities(out), nil
}

// identityLine is one line of `security find-identity -v`: an index with a
// closing parenthesis, the SHA-1, and the identity's name in double quotes.
var identityLine = regexp.MustCompile(`^\s*\d+\)\s+([0-9A-Fa-f]{40})\s+"(.*)"\s*$`)

func parseSigningIdentities(out []byte) []signingIdentity {
	var ids []signingIdentity
	for _, line := range strings.Split(string(out), "\n") {
		if m := identityLine.FindStringSubmatch(line); m != nil {
			ids = append(ids, signingIdentity{SHA1: strings.ToUpper(m[1]), Name: m[2]})
		}
	}
	return ids
}

// profileDirs are where Xcode keeps installed provisioning profiles: the
// UserData directory since Xcode 16, MobileDevice before it.
func profileDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, "Library", "Developer", "Xcode", "UserData", "Provisioning Profiles"),
		filepath.Join(home, "Library", "MobileDevice", "Provisioning Profiles"),
	}
}

// installedProfile finds the installed iOS provisioning profile with the given
// name or UUID, as PROVISIONING_PROFILE_SPECIFIER does.
func installedProfile(nameOrUUID string) (profileInfo, bool, error) {
	for _, dir := range profileDirs() {
		files, _ := filepath.Glob(filepath.Join(dir, "*.mobileprovision"))
		for _, f := range files {
			out, err := exec.Command("security", "cms", "-D", "-i", f).Output()
			if err != nil {
				continue // not a profile security can read; Xcode would skip it too
			}
			p, err := parseProfile(out)
			if err != nil {
				return profileInfo{}, false, fmt.Errorf("reading %s: %w", f, err)
			}
			if p.Name == nameOrUUID || strings.EqualFold(p.UUID, nameOrUUID) {
				return p, true, nil
			}
		}
	}
	return profileInfo{}, false, nil
}

// parseProfile reads the Name, UUID and DeveloperCertificates of a decoded
// provisioning profile (an XML property list). Only the top-level dictionary's
// keys are read; nested dictionaries, such as Entitlements, are skipped.
func parseProfile(plist []byte) (profileInfo, error) {
	var p profileInfo
	dec := xml.NewDecoder(bytes.NewReader(plist))
	depth, key := 0, ""
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return p, nil
			}
			return p, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			// depth 2 is the top-level <dict> inside <plist>; its children are at 3.
			if depth != 3 {
				if depth == 4 && t.Name.Local == "data" && key == "DeveloperCertificates" {
					var b64 string
					if err := dec.DecodeElement(&b64, &t); err != nil {
						return p, err
					}
					depth--
					der, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(b64), ""))
					if err != nil {
						return p, fmt.Errorf("DeveloperCertificates: %w", err)
					}
					sum := sha1.Sum(der)
					p.CertSHA1s = append(p.CertSHA1s, strings.ToUpper(hex.EncodeToString(sum[:])))
				}
				continue
			}
			var text string
			switch t.Name.Local {
			case "key", "string":
				if err := dec.DecodeElement(&text, &t); err != nil {
					return p, err
				}
				depth--
			}
			switch t.Name.Local {
			case "key":
				key = text
			case "string":
				switch key {
				case "Name":
					p.Name = text
				case "UUID":
					p.UUID = text
				}
			}
		case xml.EndElement:
			depth--
		}
	}
}

// lookupCertSHA1 returns the PEM of the keychain certificate with the given
// SHA-1, from `security find-certificate -a -Z -p`, which prints each
// certificate's hashes before its PEM.
func lookupCertSHA1(sha string) ([]byte, error) {
	out, err := exec.Command("security", "find-certificate", "-a", "-Z", "-p").Output()
	if err != nil {
		return nil, err
	}
	return certPEMWithSHA1(out, sha)
}

func certPEMWithSHA1(out []byte, sha string) ([]byte, error) {
	sha = strings.ToUpper(sha)
	current := ""
	var pem []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "SHA-1 hash: "):
			current = strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(line, "SHA-1 hash: ")))
		case line == "-----BEGIN CERTIFICATE-----":
			pem = []string{line}
		case line == "-----END CERTIFICATE-----":
			pem = append(pem, line)
			if current == sha {
				return []byte(strings.Join(pem, "\n") + "\n"), nil
			}
			pem = nil
		case pem != nil:
			pem = append(pem, line)
		}
	}
	return nil, fmt.Errorf("no certificate with SHA-1 %s in the keychain", sha)
}
