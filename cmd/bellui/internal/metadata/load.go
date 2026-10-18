package metadata

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/belluikit/bellui/cmd/bellui/internal/metadata/belluiapppb"
	"github.com/trendvidia/protowire-go/encoding/pxf"
)

// ManifestFile is the standard file name of the application metadata manifest.
// It holds the BelluiApp schema encoded in the textual PXF format.
const ManifestFile = "manifest.pxf"

// Load attempts to read a BelluiApp metadata from the provided reader.
// The content is decoded from the textual PXF format. If this cannot be done an
// error will be returned.
func Load(r io.Reader) (*BelluiApp, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var msg belluiapppb.BelluiApp
	if err := pxf.Unmarshal(raw, &msg); err != nil {
		return nil, err
	}

	return fromProto(&msg), nil
}

// LoadStandard attempts to read a BelluiApp metadata from the `manifest.pxf` file in the specified dir.
// If the file cannot be found or parsed an error will be returned; it names the
// file, and wraps os.ErrNotExist when there is no manifest, so a caller can
// treat a missing manifest as optional and every other error as fatal.
func LoadStandard(dir string) (*BelluiApp, error) {
	path := filepath.Join(dir, ManifestFile)
	r, err := os.Open(path)
	if err != nil {
		return nil, err // *PathError: names the file
	}

	defer r.Close()
	data, err := Load(r)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return data, nil
}
