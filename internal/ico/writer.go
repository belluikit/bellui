package ico

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"image"
	"image/draw"
	"image/png"
	"io"
)

type icondir struct {
	reserved  uint16 //lint:ignore U1000 in spec but not used
	imageType uint16
	numImages uint16
}

type icondirentry struct {
	imageWidth   uint8
	imageHeight  uint8
	numColors    uint8 //lint:ignore U1000 in spec but not used
	reserved     uint8 //lint:ignore U1000 in spec but not used
	colorPlanes  uint16
	bitsPerPixel uint16
	sizeInBytes  uint32
	offset       uint32
}

func newIcondir() icondir {
	var id icondir
	id.imageType = 1
	id.numImages = 1
	return id
}

func newIcondirentry() icondirentry {
	var ide icondirentry
	ide.colorPlanes = 1   // windows is supposed to not mind 0 or 1, but other icon files seem to have 1 here
	ide.bitsPerPixel = 32 // can be 24 for bitmap or 24/32 for png. Set to 32 for now
	ide.offset = 22       // 6 icondir + 16 icondirentry, next image will be this image size + 16 icondirentry, etc
	return ide
}

func Encode(w io.Writer, im image.Image) error {
	b := im.Bounds()
	m := image.NewRGBA(b)
	draw.Draw(m, b, im, b.Min, draw.Src)

	id := newIcondir()
	ide := newIcondirentry()

	pngbb := new(bytes.Buffer)
	pngwriter := bufio.NewWriter(pngbb)
	if err := png.Encode(pngwriter, m); err != nil {
		return err
	}
	if err := pngwriter.Flush(); err != nil {
		return err
	}
	ide.sizeInBytes = uint32(len(pngbb.Bytes()))

	bounds := m.Bounds()
	ide.imageWidth = uint8(bounds.Dx())
	ide.imageHeight = uint8(bounds.Dy())
	bb := new(bytes.Buffer)

	// Writes into bb cannot fail (bytes.Buffer grows or panics), but the
	// header is built from them, so a future change that swaps the buffer
	// should not silently lose them either.
	if err := binary.Write(bb, binary.LittleEndian, id); err != nil {
		return err
	}
	if err := binary.Write(bb, binary.LittleEndian, ide); err != nil {
		return err
	}

	// These are the writes that matter: w is the caller's destination — a
	// file being packaged, or a buffer handed to the window manager — and a
	// short write there is what produces a truncated icon.
	if _, err := w.Write(bb.Bytes()); err != nil {
		return err
	}
	if _, err := w.Write(pngbb.Bytes()); err != nil {
		return err
	}

	return nil
}
