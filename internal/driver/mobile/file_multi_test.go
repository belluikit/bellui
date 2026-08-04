package mobile

import (
	"testing"

	"github.com/belluikit/bellui/storage"

	"github.com/stretchr/testify/assert"
)

// The picker hands each URI's platform resource closer to fileReaderForURI up
// front (#255): the native reader captures it at construction, and when no
// reader is produced the closer must still run so a picker-vended resource
// (an iOS security scope) is never stranded. The desktop stub produces no
// reader, exercising exactly that path.
func TestFileReaderForURI_ReleasesResourceWithoutReader(t *testing.T) {
	released := false
	r, err := fileReaderForURI(storage.NewFileURI("/nonexistent-probe-file"), func() { released = true })
	assert.Nil(t, r)
	assert.NoError(t, err)
	assert.True(t, released, "the platform closer must run when no reader takes ownership of it")
}
