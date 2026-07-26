package metadata

import (
	"io"
	"os"
	"path/filepath"

	"github.com/belluikit/bellui/internal/metadata/belluiapppb"
	"github.com/trendvidia/protowire-go/encoding/pxf"
)

// ManifestFile is the standard file name of the application metadata manifest.
// It holds the BelluiApp schema encoded in the textual PXF format.
const ManifestFile = "manifest.pxf"

// Load attempts to read a BelluiApp metadata from the provided reader.
// The content is decoded from the textual PXF format. If this cannot be done an
// error will be returned.
//
// Unknown fields are silently ignored (DiscardUnknown). A project's
// manifest.pxf may carry packaging-tool extension fields (e.g.
// details.ios_usage_descriptions, consumed by the bellui CLI) that this runtime
// schema does not model. The runtime only reads a handful of known fields (ID,
// name, version, build, icon), so an unrecognised extension field must not fail
// the whole load — otherwise the best-effort dev-mode fallback in
// app.checkLocalMetadata would spam a parse error on every launch (#1020). The
// strict, schema-validating loader used by the packaging CLI lives in a
// separate copy (cmd/bellui/internal/metadata) and is unaffected.
func Load(r io.Reader) (*BelluiApp, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var msg belluiapppb.BelluiApp
	if err := (pxf.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(raw, &msg); err != nil {
		return nil, err
	}

	return fromProto(&msg), nil
}

// LoadStandard attempts to read a BelluiApp metadata from the `manifest.pxf` file in the specified dir.
// If the file cannot be found or parsed an error will be returned.
func LoadStandard(dir string) (*BelluiApp, error) {
	path := filepath.Join(dir, ManifestFile)
	r, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer r.Close()
	return Load(r)
}
