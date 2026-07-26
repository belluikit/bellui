package storage

import (
	"io"

	"github.com/belluikit/bellui"
)

// LoadResourceFromURI creates a new StaticResource in memory using the contents of the specified URI.
// The URI will be opened using the current driver, so valid schemas will vary from platform to platform.
// The file:// schema will always work.
func LoadResourceFromURI(u bellui.URI) (bellui.Resource, error) {
	read, err := Reader(u)
	if err != nil {
		return nil, err
	}

	defer read.Close()
	bytes, err := io.ReadAll(read)
	if err != nil {
		return nil, err
	}

	return bellui.NewStaticResource(u.Name(), bytes), nil
}

// NewLazyResourceFromURI creates a resource that loads its content via the
// storage repository system on first access. The URI will be opened using
// the current driver, so valid schemas will vary from platform to platform.
// The file:// schema will always work.
//
// The returned resource also supports streaming via [bellui.StreamingResource.Open],
// which opens the URI directly without caching the bytes.
//
// Since: 2.7
func NewLazyResourceFromURI(u bellui.URI) *bellui.LazyResource {
	return bellui.NewStreamingLazyResource(
		u.Name(),
		func() ([]byte, error) {
			read, err := Reader(u)
			if err != nil {
				return nil, err
			}
			defer read.Close()
			return io.ReadAll(read)
		},
		func() (io.ReadCloser, error) {
			return Reader(u)
		},
	)
}
