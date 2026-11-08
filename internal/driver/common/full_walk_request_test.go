package common

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// TestFullWalkAnswersTheRequestForOne: new content asks the next min-size pass
// to walk everything. A pass that walks everything anyway, as the first one
// does because nothing was measured yet, answers that request, so the frame
// after it walks only the changed paths. The request used to outlive such a
// pass, and every new content cost two full walks.
func TestFullWalkAnswersTheRequestForOne(t *testing.T) {
	test.NewTempApp(t)
	c := walkedCanvas(container.NewVBox(widget.NewLabel("a")))
	full := c.lastFullMinSize

	c.EnsureMinSize()

	assert.Equal(t, full, c.lastFullMinSize, "the frame after the first walked everything again")
}
