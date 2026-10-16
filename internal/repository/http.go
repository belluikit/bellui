package repository

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage/repository"
)

var _ repository.Repository = (*HTTPRepository)(nil)

// maxRemoteSize caps how many bytes are read from a remote resource, guarding
// against unbounded-download memory exhaustion when rendering untrusted content
// (pre-rebrand tracker #285).
const maxRemoteSize = 64 << 20 // 64 MiB

// safeClient is a hardened http.Client used for all remote repository access.
// It bounds request time and refuses to connect to link-local / multicast /
// unspecified addresses — most importantly the 169.254.169.254 cloud-metadata
// endpoint — so that fetching a resource referenced by content cannot be turned
// into a probe of those never-legitimate targets, both for the initial request
// and for any redirect, since DialContext runs for every connection (pre-rebrand tracker #285).
//
// Loopback and RFC 1918 / ULA private ranges are deliberately NOT blocked here:
// desktop apps legitimately talk to localhost and LAN services through the
// storage layer, and the no-interaction SSRF vector for untrusted markup is
// already closed by RichText.AllowRemoteContent defaulting to false.
var safeClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			Control: func(_, address string, _ syscall.RawConn) error {
				host, _, err := net.SplitHostPort(address)
				if err != nil {
					return err
				}
				if ip := net.ParseIP(host); ip != nil && isBlockedIP(ip) {
					return fmt.Errorf("refusing to connect to non-public address %s", address)
				}
				return nil
			},
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}

// isBlockedIP reports whether ip is in a range that remote content must never
// be allowed to reach: link-local (including the 169.254.169.254 cloud-metadata
// endpoint and fe80::/10), multicast, or the unspecified address.
func isBlockedIP(ip net.IP) bool {
	return ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified()
}

type remoteFile struct {
	*http.Response
	uri  bellui.URI
	body io.Reader // size-limited view of Response.Body
}

func (f *remoteFile) Close() error {
	if f.Response == nil {
		return nil
	}
	return f.Response.Body.Close()
}

func (f *remoteFile) Read(p []byte) (int, error) {
	if f.body == nil {
		return 0, nil
	}
	return f.body.Read(p)
}

func (f *remoteFile) URI() bellui.URI {
	return f.uri
}

// HTTPRepository implements a proxy for interacting with remote resources
// using golang's net/http library.
//
// This repository is suitable to handle the http:// and https:// scheme.
//
// Since: 1.0
type HTTPRepository struct{}

// NewHTTPRepository creates a new HTTPRepository instance.
// The caller needs to call repository.Register() with the result of this function.
//
// Since: 1.0
func NewHTTPRepository() *HTTPRepository {
	return &HTTPRepository{}
}

// Exists checks whether the resource at u returns a
// non "404 NOT FOUND" response header.
//
// Since: 1.0
func (r *HTTPRepository) Exists(u bellui.URI) (bool, error) {
	resp, err := safeClient.Head(u.String())
	if err != nil {
		return false, err
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	return true, nil
}

// Reader provides a interface for reading the body of the response received
// from the request to u.
//
// Since: 1.0
func (r *HTTPRepository) Reader(u bellui.URI) (bellui.URIReadCloser, error) {
	resp, err := safeClient.Get(u.String())
	if err != nil {
		return &remoteFile{uri: u}, err
	}
	return &remoteFile{Response: resp, uri: u, body: io.LimitReader(resp.Body, maxRemoteSize)}, nil
}

// CanRead makes a HEAD HTTP request to analyse the headers received
// from the remote server.
// Any response status code apart from 2xx is considered to be invalid.
//
// Since: 1.0
func (r *HTTPRepository) CanRead(u bellui.URI) (bool, error) {
	resp, err := safeClient.Head(u.String())
	if err != nil {
		return false, err
	}
	resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode > http.StatusIMUsed {
		return false, errors.New("remote server did not return a successful response")
	}

	return true, nil
}

// Destroy satisfies the repository.Repository interface.
//
// Since: 1.0
func (r *HTTPRepository) Destroy(string) {
	// do nothing
}
