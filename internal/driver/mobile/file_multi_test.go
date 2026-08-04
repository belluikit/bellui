package mobile

import (
	"errors"
	"io"
	"testing"

	"github.com/belluikit/bellui"
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

type closeRecorder struct {
	closed *bool
}

func (r *closeRecorder) Read([]byte) (int, error) { return 0, io.EOF }
func (r *closeRecorder) Close() error             { *r.closed = true; return nil }
func (r *closeRecorder) URI() bellui.URI          { return nil }

// A failed open must not deliver a partial set (#255): every reader already
// built is closed, every closer not yet reached is run, and the failing URI's
// own closer is the opener's to run — each platform resource is released
// exactly once, none twice.
func TestMultiReadersForURIs_FirstFailureReleasesEverything(t *testing.T) {
	var closerRan [3]bool
	closers := []func(){
		func() { closerRan[0] = true },
		func() { closerRan[1] = true },
		func() { closerRan[2] = true },
	}

	firstClosed := false
	failure := errors.New("open failed")
	readers, err := multiReadersForURIs([]string{"a", "b", "c"}, closers, func(uri string, closer func()) (bellui.URIReadCloser, error) {
		switch uri {
		case "a":
			return &closeRecorder{closed: &firstClosed}, nil
		case "b":
			closer() // the opener's contract: a failed open runs its own closer
			return nil, failure
		default:
			t.Errorf("opened %q after the failure", uri)
			return nil, nil
		}
	})

	assert.Nil(t, readers)
	assert.Equal(t, failure, err)
	assert.True(t, firstClosed, "the already-built reader must be closed")
	assert.False(t, closerRan[0], "the first closer belongs to its reader now — running it here would release twice")
	assert.True(t, closerRan[1], "the failing URI's closer runs via the opener")
	assert.True(t, closerRan[2], "the unreached closer must be released")
}
