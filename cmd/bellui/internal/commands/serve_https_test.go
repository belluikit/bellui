package commands

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The certificate is one a browser can accept for each address a phone
// would use: localhost and this machine's network addresses (#1359).
func TestSelfSignedCertCoversTheServedAddresses(t *testing.T) {
	lan := net.ParseIP("192.168.1.20")
	now := time.Now()
	cert, err := selfSignedCert([]net.IP{lan}, now)
	require.NoError(t, err)

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	for _, host := range []string{"localhost", "127.0.0.1", "::1", "192.168.1.20"} {
		_, err := leaf.Verify(x509.VerifyOptions{DNSName: host, Roots: pool, CurrentTime: now, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}})
		assert.NoError(t, err, host)
	}
	_, err = leaf.Verify(x509.VerifyOptions{DNSName: "192.168.1.21", Roots: pool, CurrentTime: now})
	assert.Error(t, err, "an address it was not made for")
	_, err = leaf.Verify(x509.VerifyOptions{DNSName: "localhost", Roots: pool, CurrentTime: now.Add(31 * 24 * time.Hour)})
	assert.Error(t, err, "it expires")

	again, err := selfSignedCert([]net.IP{lan}, now)
	require.NoError(t, err)
	assert.NotEqual(t, certFingerprint(cert), certFingerprint(again), "each serve makes its own key")
	assert.Len(t, certFingerprint(cert), 32*3-1)
}

// lockedBuffer is an io.Writer that a test can read while run writes to it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// startRun runs s.run on a listener at every IPv4 address with handler, and
// returns its port and its output, plus stop, which ends the run and returns
// its error, and wait, which returns its error once it ends by itself.
func startRun(t *testing.T, s *Server, handler http.Handler) (port int, out *lockedBuffer, stop, wait func() error) {
	t.Helper()
	ln, err := net.Listen("tcp4", "0.0.0.0:0")
	require.NoError(t, err)
	out = &lockedBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.run(ctx, ln, handler, out) }()
	var once sync.Once
	var runErr error
	wait = func() error {
		once.Do(func() {
			select {
			case runErr = <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("run did not return")
			}
		})
		return runErr
	}
	stop = func() error {
		cancel()
		return wait()
	}
	t.Cleanup(func() { _ = stop() })
	return ln.Addr().(*net.TCPAddr).Port, out, stop, wait
}

// serve --https answers over TLS with its own certificate, prints where, and
// stops cleanly.
func TestServeHTTPSAnswersOverTLS(t *testing.T) {
	s := &Server{appData: &appData{AppID: "com.example.hello"}, https: true}
	port, out, stop, _ := startRun(t, s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "hello")
	}))

	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, Timeout: 10 * time.Second}
	var resp *http.Response
	require.Eventually(t, func() bool {
		var err error
		resp, err = client.Get("https://127.0.0.1:" + strconv.Itoa(port) + "/")
		return err == nil
	}, 5*time.Second, 20*time.Millisecond)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	assert.Equal(t, "hello", string(body))
	require.NotNil(t, resp.TLS)
	served := resp.TLS.PeerCertificates[0]

	printed := out.String()
	assert.Contains(t, printed, "Serving com.example.hello at: https://localhost:"+strconv.Itoa(port)+"\n")
	if lan := nonLoopbackIPv4(); lan != "" {
		assert.Contains(t, printed, "  on the network: https://"+lan+":"+strconv.Itoa(port)+"\n")
	}
	assert.Contains(t, printed, "Its SHA-256 fingerprint: "+certFingerprint(tls.Certificate{Certificate: [][]byte{served.Raw}}),
		"the printed fingerprint is the served certificate's")

	plain, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/")
	require.NoError(t, err)
	_ = plain.Body.Close()
	assert.Equal(t, http.StatusBadRequest, plain.StatusCode, "plain http gets no page")
	assert.NoError(t, stop())
}

// The point of --https: a page served with the self-signed certificate at a
// network address is a secure context, so Chrome offers it WebGPU and the
// page no longer names the insecure page (#1358, #1359). Chrome here takes
// the certificate by flag, which stands in for the user accepting the
// warning.
func TestServeHTTPSIsASecurePageInChrome(t *testing.T) {
	chrome := findChrome()
	if chrome == "" {
		t.Skip("needs Chrome")
	}
	lan := nonLoopbackIPv4()
	if lan == "" {
		t.Skip("needs a non-loopback IPv4 address")
	}
	page := []byte(renderIndex(t, true))
	// WebGPU is stubbed away: the secure check comes first, and on CI's macOS
	// machines the adapter request never settles.
	stubbed := bytes.Replace(page, []byte("<head>"), []byte("<head><script>"+stubNoWebGPU+"</script>"), 1)
	probe := []byte(`<html><body><div id="action"><script>document.write("secure=" + window.isSecureContext)</script></div></body></html>`)
	s := &Server{appData: &appData{}, https: true}
	port, _, _, _ := startRun(t, s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/probe" {
			_, _ = w.Write(probe)
			return
		}
		_, _ = w.Write(stubbed)
	}))
	base := "https://" + net.JoinHostPort(lan, strconv.Itoa(port))

	warmChrome(t, chrome)
	shown := actionText(t, dumpPage(t, chrome, t.TempDir(), base+"/probe", "--ignore-certificate-errors"))
	assert.Contains(t, shown, "secure=true")

	shown = actionText(t, dumpPage(t, chrome, t.TempDir(), base+"/", "--ignore-certificate-errors"))
	assert.False(t, strings.Contains(shown, insecurePageMessage), "shown: %q", shown)
	assert.Contains(t, shown, noWebGPUMessage, "a secure page with no WebGPU blames the browser")
}
