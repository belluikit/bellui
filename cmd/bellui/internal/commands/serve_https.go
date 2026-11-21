package commands

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"time"
)

// selfSignedCert makes a certificate for serve --https, valid for localhost
// and for each address in ips, so a phone can open the app at this machine's
// network address. Browsers offer WebGPU only to a secure page (#1358); a
// self-signed one still shows a warning the user has to click through
// (#1359). It lives only as long as the serve that made it.
func selfSignedCert(ips []net.IP, now time.Time) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}
	names := []string{"localhost"}
	if host, err := os.Hostname(); err == nil && host != "" && host != "localhost" {
		names = append(names, host)
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "bellui serve", Organization: []string{"bellui serve (self-signed)"}},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(30 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     names,
		IPAddresses:  append([]net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}, ips...),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, nil
}

// certFingerprint is the SHA-256 of a certificate, in the colon-separated form
// browsers show in their certificate viewer, so the user can check that the
// page behind the warning is this serve.
func certFingerprint(cert tls.Certificate) string {
	sum := sha256.Sum256(cert.Certificate[0])
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(parts, ":")
}

// networkIPs returns this machine's addresses another device could reach:
// not loopback, and not IPv6 link-local, which needs a zone a URL cannot
// carry portably.
func networkIPs() []net.IP {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var ips []net.IP
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() || ipnet.IP.IsLinkLocalUnicast() || ipnet.IP.IsLinkLocalMulticast() {
			continue
		}
		ips = append(ips, ipnet.IP)
	}
	return ips
}
