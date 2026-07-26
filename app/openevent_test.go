package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyOpenArg(t *testing.T) {
	cases := []struct {
		arg    string
		ok     bool
		isURL  bool
		scheme string // for URL cases
	}{
		{"http://example.com/x", true, true, "http"},
		{"https://example.com", true, true, "https"},
		{"myapp://open?id=1", true, true, "myapp"},
		{"mailto:a@b.com", true, true, "mailto"},
		{"/home/user/doc.txt", true, false, ""},
		{"file:///home/user/doc.txt", true, false, ""},
		{`C:\Users\me\a.txt`, true, false, ""},
		{"C:/Users/me/a.txt", true, false, ""},
		{"relative/path.txt", true, false, ""},
		{"doc.txt", true, false, ""},
		{"-flag", false, false, ""},
		{"--flag=x", false, false, ""},
		{"", false, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.arg, func(t *testing.T) {
			p, ok := classifyOpenArg(tc.arg)
			require.Equal(t, tc.ok, ok)
			if !ok {
				return
			}
			if tc.isURL {
				require.NotNil(t, p.link, "expected a URL payload")
				assert.Nil(t, p.uri)
				assert.Equal(t, tc.scheme, p.link.Scheme)
			} else {
				require.NotNil(t, p.uri, "expected a file payload")
				assert.Nil(t, p.link)
			}
		})
	}
}

func TestIsWindowsDrivePath(t *testing.T) {
	assert.True(t, isWindowsDrivePath(`C:\x`))
	assert.True(t, isWindowsDrivePath("C:/x"))
	assert.True(t, isWindowsDrivePath("c:"))
	assert.False(t, isWindowsDrivePath("http://x")) // multi-char scheme
	assert.False(t, isWindowsDrivePath("1:/x"))     // not a letter
	assert.False(t, isWindowsDrivePath("mailto:x")) // colon but not drive
}
