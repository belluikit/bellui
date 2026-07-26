package auth

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleCallback_EscapesError(t *testing.T) {
	cb := &callbackServer{codeCh: make(chan callbackResult, 1)}

	req := httptest.NewRequest("GET",
		"/callback?error=access_denied&error_description=<script>alert(1)</script>", nil)
	rec := httptest.NewRecorder()

	cb.handleCallback(rec, req)

	body := rec.Body.String()
	assert.NotContains(t, body, "<script>alert(1)</script>", "raw error must not be reflected")
	assert.Contains(t, body, "&lt;script&gt;", "error must be HTML-escaped")

	// The error is still delivered to the waiter unescaped for programmatic use.
	res := <-cb.codeCh
	assert.Error(t, res.Err)
	assert.True(t, strings.Contains(res.Err.Error(), "<script>"))
}

func TestHandleCallback_Success(t *testing.T) {
	cb := &callbackServer{codeCh: make(chan callbackResult, 1)}

	req := httptest.NewRequest("GET", "/callback?code=abc123&state=s1", nil)
	rec := httptest.NewRecorder()

	cb.handleCallback(rec, req)

	res := <-cb.codeCh
	assert.NoError(t, res.Err)
	assert.Equal(t, "abc123", res.Code)
	assert.Equal(t, "s1", res.State)
}
