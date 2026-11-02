package source

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/belluikit/goffmpeg"
)

// httpsInGo routes an https URL through httpReader instead of FFmpeg's own
// protocol handler. The Android FFmpeg libraries have no TLS
// (belluikit/goffmpeg-libs#10): every https open failed with "failed to open
// decoder: -1" (#871). Go's client has TLS and reads Android's system roots, so
// apps need no CA bundle. A variable so the tests can take this path anywhere.
var httpsInGo = runtime.GOOS == "android"

// errHLSOverReader reports an https playlist where only the Go reader can carry
// https. A playlist names its segments by URL, and a demuxer reading bytes has
// no protocol handler to fetch them with (TestHLSViaReaderFails).
var errHLSOverReader = errors.New("mmedia: https HLS cannot play on this platform: " +
	"its FFmpeg has no TLS, and a playlist cannot be read through Go " +
	"(belluikit/goffmpeg-libs#10)")

// The reader rides out a network that drops or stalls as httpOptions makes
// FFmpeg do (#839): httpReadTimeout turns a silent server into a drop, and a
// drop is resumed from the byte it stopped at, after each of httpRetryDelays in
// turn, which add up to about reconnect_delay_max.
//
// An open is retried the same way, which FFmpeg's is not (#873): on a phone
// the network goes away for a moment more often than a link dies, and two
// opens failed together in one such moment. httpOpenTimeout bounds the whole
// open, retries included, so a link that never answers still fails in bounded
// time.
var (
	httpReadTimeout = 5 * time.Second
	httpRetryDelays = []time.Duration{250 * time.Millisecond, 500 * time.Millisecond, time.Second}
	httpOpenTimeout = 10 * time.Second
)

// httpSkipLimit is how far ahead a seek reads through the open response
// rather than starting a new request. FFmpeg seeks a few kilobytes forward all
// the time, over a gap between packets, and a round trip costs more than that.
var httpSkipLimit int64 = 256 << 10

// httpClient makes the reader's requests; a test swaps in its TLS server's.
var httpClient = http.DefaultClient

// UseGoHTTPSForProbes reads https through Go on any platform, with requests
// made by c, until restore is called. It is for hack/httpsprobe, which counts
// what the reader fetches; tests set the variables directly.
func UseGoHTTPSForProbes(c *http.Client) (restore func()) {
	oldGo, oldClient := httpsInGo, httpClient
	httpsInGo, httpClient = true, c
	return func() { httpsInGo, httpClient = oldGo, oldClient }
}

// httpReader is an io.ReadSeeker over an http(s) URL. It streams one response
// from the read position, and starts another with a Range header only when a
// seek leaves it, so a file plays as one request plus one per real seek. A
// server that ignores Range is read through from the start up to the position.
//
// An open response keeps working after a signed link expires, since the
// signature is checked once, per request. A request made after that, for a seek
// or a reconnect, fails, and Err says so.
//
// Read and Seek are called from the decoding goroutine; Close may be called
// from any.
type httpReader struct {
	url    string
	client *http.Client
	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex
	pos     int64
	size    int64 // -1 when the server did not say
	body    io.ReadCloser
	bodyPos int64 // where body's next byte is in the file
	stop    context.CancelFunc
	head    []byte // bytes already read from body, ahead of bodyPos
	err     error  // why reading stopped, for Err
}

// newHTTPReader opens url and reads its first bytes, so that a link that does
// not resolve fails here, as FFmpeg's open does, and a playlist is recognised.
func newHTTPReader(rawURL string) (*httpReader, error) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &httpReader{url: rawURL, client: httpClient, ctx: ctx, cancel: cancel, size: -1}
	deadline := time.Now().Add(httpOpenTimeout)
	for attempt := 0; ; attempt++ {
		err := r.start(deadline)
		if err == nil {
			return r, nil
		}
		if !openRetryable(err) || attempt == len(httpRetryDelays) ||
			time.Until(deadline) <= httpRetryDelays[attempt] {
			r.Close()
			return nil, err
		}
		time.Sleep(httpRetryDelays[attempt])
	}
}

// start opens the response at 0 and reads its first bytes, each within the
// read timeout and before deadline.
func (r *httpReader) start(deadline time.Time) error {
	if err := r.open(0, min(httpReadTimeout, time.Until(deadline))); err != nil {
		return err
	}
	var head [7]byte
	var n int
	var err error
	withTimeout(min(httpReadTimeout, time.Until(deadline)), r.stop, func() { n, err = io.ReadFull(r.body, head[:]) })
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		r.closeBody()
		return fmt.Errorf("mmedia: reading %s: %w", redactURL(r.url), err)
	}
	if bytes.Equal(head[:n], []byte("#EXTM3U")) {
		return errHLSOverReader
	}
	r.head = append([]byte(nil), head[:n]...)
	return nil
}

// openRetryable is true of an open that failed on the network or on a server
// error, which a moment can fix. A certificate the client does not trust, a
// client error such as a 404 or an expired signature, and a playlist stay.
func openRetryable(err error) bool {
	var se *statusError
	if errors.As(err, &se) {
		return se.retryable()
	}
	var cert *tls.CertificateVerificationError
	if errors.As(err, &cert) {
		return false
	}
	var ue *url.Error
	var ne net.Error
	return errors.As(err, &ue) || errors.As(err, &ne) || errors.Is(err, context.Canceled)
}

// open starts a response at off, replacing any open one; timeout bounds the
// wait for its headers.
func (r *httpReader) open(off int64, timeout time.Duration) error {
	r.closeBody()
	ctx, stop := context.WithCancel(r.ctx)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url, nil)
	if err != nil {
		stop()
		return err
	}
	req.Header.Set("Range", "bytes="+strconv.FormatInt(off, 10)+"-")
	// A compressed body would make the offsets the server's, not the file's.
	req.Header.Set("Accept-Encoding", "identity")
	var resp *http.Response
	withTimeout(timeout, stop, func() { resp, err = r.client.Do(req) })
	if err != nil {
		stop()
		var ue *url.Error
		if errors.As(err, &ue) {
			ue.URL = redactURL(ue.URL)
		}
		return err
	}

	switch resp.StatusCode {
	case http.StatusPartialContent:
		start, total, ok := parseContentRange(resp.Header.Get("Content-Range"))
		if !ok || start != off {
			resp.Body.Close()
			stop()
			return fmt.Errorf("mmedia: %s answered range %d- with %q",
				redactURL(r.url), off, resp.Header.Get("Content-Range"))
		}
		if total >= 0 {
			r.size = total
		}
	case http.StatusOK:
		if resp.ContentLength >= 0 {
			r.size = resp.ContentLength
		}
		// The server ignored Range and sent the file from its start.
		if off > 0 {
			if _, err := io.CopyN(io.Discard, resp.Body, off); err != nil {
				resp.Body.Close()
				stop()
				return err
			}
		}
	case http.StatusRequestedRangeNotSatisfiable:
		// At or past the end: nothing to read, which Read reports as EOF.
		resp.Body.Close()
		r.body, r.bodyPos, r.stop = http.NoBody, off, stop
		return nil
	default:
		resp.Body.Close()
		stop()
		return &statusError{url: r.url, code: resp.StatusCode}
	}
	r.body, r.bodyPos, r.stop = resp.Body, off, stop
	return nil
}

// withTimeout runs f, cancelling the request through stop if it takes longer
// than d, which is httpReadTimeout but in an open near its deadline. f is a request or a read, which a server that goes
// silent without closing would otherwise block forever.
func withTimeout(d time.Duration, stop context.CancelFunc, f func()) {
	t := time.AfterFunc(d, stop)
	defer t.Stop()
	f()
}

func (r *httpReader) closeBody() {
	if r.body != nil {
		r.body.Close()
		r.body = nil
	}
	if r.stop != nil {
		r.stop()
		r.stop = nil
	}
	r.head = nil
}

func (r *httpReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ctx.Err() != nil {
		return 0, errReaderClosed
	}
	if len(p) == 0 {
		return 0, nil
	}
	if r.size >= 0 && r.pos >= r.size {
		return 0, io.EOF
	}

	for attempt := 0; ; {
		n, err := r.readOnce(p)
		if n > 0 {
			r.err = nil
			return n, nil
		}
		if err == nil {
			continue // a reader may return (0, nil); goffmpeg would take 0 as the end
		}
		if errors.Is(err, io.EOF) && (r.size < 0 || r.pos >= r.size) {
			return 0, io.EOF
		}
		var se *statusError
		if r.ctx.Err() != nil || (errors.As(err, &se) && !se.retryable()) || attempt == len(httpRetryDelays) {
			if err == nil || errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}
			r.err = err
			return 0, err
		}
		r.closeBody()
		select {
		case <-time.After(httpRetryDelays[attempt]):
		case <-r.ctx.Done():
		}
		attempt++
	}
}

// readOnce reads from the response at pos, opening one there if there is none.
func (r *httpReader) readOnce(p []byte) (int, error) {
	if r.body == nil || r.bodyPos != r.pos {
		if err := r.seekBody(); err != nil {
			return 0, err
		}
	}
	if len(r.head) > 0 {
		n := copy(p, r.head)
		r.head = r.head[n:]
		r.pos += int64(n)
		r.bodyPos = r.pos
		return n, nil
	}
	var n int
	var err error
	withTimeout(httpReadTimeout, r.stop, func() { n, err = r.body.Read(p) })
	r.pos += int64(n)
	r.bodyPos = r.pos
	if err != nil && !errors.Is(err, io.EOF) {
		r.closeBody()
	}
	return n, err
}

// seekBody brings the response to pos: by reading through it when pos is a
// short way ahead, otherwise with a new request.
func (r *httpReader) seekBody() error {
	if r.body != nil && r.pos > r.bodyPos && r.pos-r.bodyPos <= httpSkipLimit {
		skip := r.pos - r.bodyPos
		if h := int64(len(r.head)); h > 0 {
			k := min(h, skip)
			r.head = r.head[k:]
			skip -= k
		}
		var err error
		withTimeout(httpReadTimeout, r.stop, func() { _, err = io.CopyN(io.Discard, r.body, skip) })
		if err == nil {
			r.bodyPos = r.pos
			return nil
		}
	}
	return r.open(r.pos, httpReadTimeout)
}

func (r *httpReader) Seek(offset int64, whence int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var pos int64
	switch whence {
	case io.SeekStart:
		pos = offset
	case io.SeekCurrent:
		pos = r.pos + offset
	case io.SeekEnd:
		if r.size < 0 {
			return 0, errors.New("mmedia: the server did not say how long the stream is")
		}
		pos = r.size + offset
	default:
		return 0, errors.New("mmedia: bad whence")
	}
	if pos < 0 {
		return 0, errors.New("mmedia: seek before the start")
	}
	// Nothing is fetched until the next Read, so a seek that FFmpeg follows
	// with another costs no request.
	r.pos = pos
	return pos, nil
}

// Err reports why reading stopped, which goffmpeg cannot pass on: the reader
// callback reports any failure to FFmpeg as -1.
func (r *httpReader) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

func (r *httpReader) unsized() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.size < 0
}

// Close ends any request in flight, which unblocks a Read waiting on one.
func (r *httpReader) Close() error {
	r.cancel()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closeBody()
	return nil
}

var errReaderClosed = errors.New("mmedia: reader is closed")

// statusError is a response that is not the media.
type statusError struct {
	url  string
	code int
}

func (e *statusError) Error() string {
	msg := fmt.Sprintf("mmedia: %s: %s", redactURL(e.url), http.StatusText(e.code))
	if e.code == http.StatusForbidden || e.code == http.StatusUnauthorized {
		msg += " (a signed link may have expired)"
	}
	return fmt.Sprintf("%s (%d)", msg, e.code)
}

// retryable is true of a server error, which a moment can fix; a client error,
// such as an expired signature, stays.
func (e *statusError) retryable() bool {
	return e.code >= 500 || e.code == http.StatusTooManyRequests
}

// parseContentRange reads "bytes start-end/total"; total is -1 for "*".
func parseContentRange(v string) (start, total int64, ok bool) {
	v, found := strings.CutPrefix(v, "bytes ")
	if !found {
		return 0, 0, false
	}
	span, size, found := strings.Cut(v, "/")
	if !found {
		return 0, 0, false
	}
	first, _, found := strings.Cut(span, "-")
	if !found {
		return 0, 0, false
	}
	start, err := strconv.ParseInt(first, 10, 64)
	if err != nil {
		return 0, 0, false
	}
	if size == "*" {
		return start, -1, true
	}
	total, err = strconv.ParseInt(size, 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return start, total, true
}

// redactURL drops the query, which in a signed link is the credential.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "the link"
	}
	u.RawQuery, u.Fragment, u.User = "", "", nil
	return u.String()
}

// readerDecoder is a decoder over an httpReader, which it closes with itself.
type readerDecoder struct {
	goffmpeg.IDecoder
	r *httpReader
}

func (d *readerDecoder) Close() {
	d.IDecoder.Close()
	d.r.Close()
}

// readErr is the reader's error, when there is one, in place of the decoder's
// opaque one. With none, a server that sent no length is the likely cause: the
// stream could not seek, and a file with its index at the end cannot play
// forward-only. FFmpeg's own http fails on such a server as well, with -5.
func (d *readerDecoder) readErr(err error) error {
	if rerr := d.r.Err(); rerr != nil {
		return fmt.Errorf("%w (decoder: %v)", rerr, err)
	}
	if d.r.unsized() {
		return fmt.Errorf("mmedia: %s sent no length, so it could only be read "+
			"forward, without seeking (decoder: %w)", redactURL(d.r.url), err)
	}
	return err
}

// openHTTPSInGo opens url through httpReader; see httpsInGo.
func openHTTPSInGo(url string) (goffmpeg.IDecoder, error) {
	r, err := newHTTPReader(url)
	if err != nil {
		return nil, err
	}
	dec, err := goffmpeg.OpenReader(r)
	if err != nil {
		r.Close()
		if rerr := r.Err(); rerr != nil {
			return nil, rerr
		}
		return nil, err
	}
	return &readerDecoder{IDecoder: dec, r: r}, nil
}
