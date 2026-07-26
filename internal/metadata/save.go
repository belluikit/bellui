package metadata

import (
	"io"
	"os"
	"path/filepath"

	"github.com/trendvidia/protowire-go/encoding/pxf"
)

// Save attempts to write a BelluiApp metadata to the provided writer.
// The content is encoded in the textual PXF format. If the encoding fails an
// error will be returned.
func Save(f *BelluiApp, w io.Writer) error {
	data, err := pxf.Marshal(f.toProto())
	if err != nil {
		return err
	}

	_, err = w.Write(data)
	return err
}

// SaveStandard attempts to save a BelluiApp metadata to the `manifest.pxf` file in the specified dir.
// If the file cannot be written or encoding fails an error will be returned.
func SaveStandard(f *BelluiApp, dir string) error {
	path := filepath.Join(dir, ManifestFile)
	w, err := os.Create(path)
	if err != nil {
		return err
	}

	defer w.Close()
	return Save(f, w)
}
