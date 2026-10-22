package pxfmigrate

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsJSON(t *testing.T) {
	assert.True(t, IsJSON([]byte(`{"a":1}`), '{'))
	assert.True(t, IsJSON([]byte(" \t\r\n[1]"), '['))
	assert.False(t, IsJSON([]byte(`[1]`), '{'))
	assert.False(t, IsJSON([]byte(`args = ["x"]`), '['))
	assert.False(t, IsJSON(nil, '['))
	assert.False(t, IsJSON([]byte(" \n"), '{'))
}

// ValidUTF8 gives what a JSON round trip gives, which is what the JSON formats
// before #523 stored.
func TestValidUTF8_MatchesJSON(t *testing.T) {
	for _, s := range []string{"", "plain", "café", "caf\xe9", "\xff\xfe", "a\xc3", "\xe2\x82", "ok\xffok\xc3\xa9"} {
		data, err := json.Marshal(s)
		require.NoError(t, err)
		var want string
		require.NoError(t, json.Unmarshal(data, &want))
		assert.Equal(t, want, ValidUTF8(s), "%q", s)
	}
}
