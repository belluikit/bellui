package analytics

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// httpClient is the shared client for vendor sinks: a bounded timeout so a
// flush can never hang the worker indefinitely.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// PostJSONGzip marshals payload to JSON, gzips it, and POSTs it to url with the
// given headers (Content-Type: application/json and Content-Encoding: gzip are
// set automatically). It is the shared transport for the vendor [Sink]
// subpackages: one gzipped request, an error on any non-2xx response. Bodies
// are drained and closed so connections are reused.
func PostJSONGzip(url string, headers map[string]string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(raw); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("analytics: unexpected status %s", resp.Status)
	}
	return nil
}
