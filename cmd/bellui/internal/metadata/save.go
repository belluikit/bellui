package metadata

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

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

// BumpBuildStandard increments `details.build` by one in the `manifest.pxf` file
// in dir, writing the result back with a format-preserving edit: only the build
// number's value changes, and every comment, blank line, key ordering and
// spacing quirk the author put there round-trips untouched. (A full
// re-serialize via SaveStandard would discard all of that on every package —
// see the pxf Rewriter.) If dir has no manifest there is nothing to bump and it
// is a no-op.
func BumpBuildStandard(dir string) error {
	path := filepath.Join(dir, ManifestFile)
	src, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil // no metadata to update
	}
	if err != nil {
		return err
	}

	cur, err := Load(bytes.NewReader(src))
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	r, err := pxf.NewRewriter(src)
	if err != nil {
		return err
	}
	if err := r.Set("details.build", &pxf.IntVal{Raw: strconv.Itoa(cur.Details.Build + 1)}); err != nil {
		return err
	}
	out, err := r.Bytes()
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}
