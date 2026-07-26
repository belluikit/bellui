package textbuf

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// fuzzInsertions is the palette of insertion texts a fuzz op can pick from:
// empty, ASCII, newlines, multi-byte, emoji and longer mixed content.
var fuzzInsertions = []string{
	"",
	"x",
	"\n",
	"é",
	"界\n",
	"🙂",
	"a\nbб\n",
	"long insertion with several words and a trailing newline\n",
}

// applyFuzzOps decodes ops (4 bytes each: low, low, high-span, insertion
// selector) into splices applied to both the Buffer and the oracle, comparing
// full state after each. Positions are taken modulo the current length so any
// byte string is a valid script.
func applyFuzzOps(t *testing.T, initial string, ops []byte) {
	b := FromString(initial)
	m := &model{s: string([]rune(initial))} // normalized like the Buffer

	for len(ops) >= 4 {
		n := len([]rune(m.s))
		low := (int(ops[0])<<8 | int(ops[1])) % (n + 1)
		high := low + int(ops[2])%(n-low+1)
		ins := fuzzInsertions[int(ops[3])%len(fuzzInsertions)]
		ops = ops[4:]

		b.Splice(low, high, []rune(ins))
		m.splice(low, high, []rune(ins))

		require.Equal(t, m.s, b.String())
		require.Equal(t, len([]rune(m.s)), b.Len())
		require.Equal(t, m.lineStarts(), bufLineStarts(b))
	}
	checkAgainstModel(t, b, m)
	checkInvariants(t, b)
}

func FuzzSplice(f *testing.F) {
	f.Add("", []byte{0, 0, 0, 1})
	f.Add("hello\nworld\n", []byte{0, 3, 2, 2, 0, 0, 5, 4, 0, 9, 9, 0})
	f.Add("aé界b\ncд\nz", []byte{0, 1, 3, 3, 0, 4, 0, 6, 0, 0, 200, 7})
	f.Add("line one\nline two\nline three\n", []byte{0, 8, 1, 2, 0, 17, 1, 0, 0, 0, 255, 5})
	f.Fuzz(func(t *testing.T, initial string, ops []byte) {
		if len(initial) > 1<<16 || len(ops) > 1<<10 {
			t.Skip("bounded input")
		}
		applyFuzzOps(t, initial, ops)
	})
}
