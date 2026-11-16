package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPressedWord_Extend(t *testing.T) {
	rows := [][]rune{[]rune("alpha beta  gamma"), []rune("delta epsilon")}
	rowText := func(r int) []rune { return rows[r] }
	w := pressedWord{}
	w.set(0, 6, 10) // "beta"

	for name, tt := range map[string]struct {
		row, col               int
		aRow, aCol, cRow, cCol int
	}{
		"inside the word":         {0, 8, 0, 6, 0, 10},
		"on its start":            {0, 6, 0, 6, 0, 10},
		"on its end":              {0, 10, 0, 6, 0, 10},
		"into the next word":      {0, 14, 0, 6, 0, 17},
		"into the word before":    {0, 2, 0, 10, 0, 0},
		"on the space before":     {0, 5, 0, 10, 0, 6},
		"on the space after":      {0, 11, 0, 6, 0, 10},
		"past the spaces after":   {0, 12, 0, 6, 0, 17},
		"a row below, mid-word":   {1, 8, 0, 6, 1, 13},
		"a row below, first word": {1, 1, 0, 6, 1, 5},
	} {
		t.Run(name, func(t *testing.T) {
			aRow, aCol, cRow, cCol := w.extend(tt.row, tt.col, rowText)
			assert.Equal(t, [4]int{tt.aRow, tt.aCol, tt.cRow, tt.cCol}, [4]int{aRow, aCol, cRow, cCol})
		})
	}
}
