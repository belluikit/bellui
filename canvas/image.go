package canvas

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg" // avoid users having to import when using image widget
	_ "image/png"  // avoid the same for PNG images
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/scale"
	"github.com/belluikit/bellui/internal/svg"
	"github.com/belluikit/bellui/storage"
)

// ImageFill defines the different type of ways an image can stretch to fill its space.
type ImageFill int

const (
	// ImageFillStretch will scale the image to match the Size() values.
	// This is the default and does not maintain aspect ratio.
	ImageFillStretch ImageFill = iota
	// ImageFillContain makes the image fit within the object Size(),
	// centrally and maintaining aspect ratio.
	// There may be transparent sections top and bottom or left and right.
	ImageFillContain // (Fit)
	// ImageFillOriginal ensures that the container grows to the pixel dimensions
	// required to fit the original image. The aspect of the image will be maintained so,
	// as with ImageFillContain there may be transparent areas around the image.
	// Note that the minSize may be smaller than the image dimensions if scale > 1.
	ImageFillOriginal

	// ImageFillCover maintains the image aspect ratio whilst filling the space.
	// The image content will be centered on the available space meaning that an equal amount of top and bottom
	// or left and right will be clipped if the output aspect ratio does not match the source image.
	// Since: 1.0
	ImageFillCover
)

// ImageScale defines the different scaling filters used to scaling images
type ImageScale int32

const (
	// ImageScaleSmooth will scale the image using ApproxBiLinear filter (or GL equivalent)
	ImageScaleSmooth ImageScale = iota
	// ImageScalePixels will scale the image using NearestNeighbor filter (or GL equivalent)
	ImageScalePixels
	// ImageScaleFastest will scale the image using hardware GPU if available
	//
	// Since: 1.0
	ImageScaleFastest
)

// Declare conformity with CanvasObject interface
var _ core.CanvasObject = (*Image)(nil)

// Image describes a drawable image area that can render in a Bellui canvas
// The image may be a vector or a bitmap representation, it will fill the area.
// The fill mode can be changed by setting FillMode to a different ImageFill.
type Image struct {
	baseObject

	propMu         sync.RWMutex // protects aspect, icon, isSVG, previousRender, and snapshots of File/Resource/Image
	aspect         float32
	icon           *svg.Decoder
	isSVG          bool
	previousRender bool // did we successfully draw before? if so a nil content will need a reset

	// one of the following sources will provide our image data
	File     string          // Load the image from a file
	Resource bellui.Resource // Load the image from an in-memory resource
	Image    image.Image     // Specify a loaded image to use in this canvas object

	Translucency float64    // Set a translucency value > 0.0 to fade the image
	FillMode     ImageFill  // Specify how the image should expand to fill or fit the available space
	ScaleMode    ImageScale // Specify the type of scaling interpolation applied to the image

	// CornerRadius specifies a radius to apply to round corners of the image.
	//
	// Since: 1.0
	CornerRadius float32
}

// Alpha is a convenience function that returns the alpha value for an image
// based on its Translucency value. The result is 1.0 - Translucency.
func (i *Image) Alpha() float64 {
	return 1.0 - i.Translucency
}

// Aspect will return the original content aspect after it was last refreshed.
// Aspect is safe for concurrent use.
//
// Since: 1.0
func (i *Image) Aspect() float32 {
	i.propMu.RLock()
	a := i.aspect
	i.propMu.RUnlock()
	if a == 0 {
		i.Refresh()
		i.propMu.RLock()
		a = i.aspect
		i.propMu.RUnlock()
	}
	return a
}

// Hide will set this image to not be visible
func (i *Image) Hide() {
	i.baseObject.Hide()

	repaint(i)
}

// MinSize returns the specified minimum size, if set, or {1, 1} otherwise.
func (i *Image) MinSize() geo.Size {
	i.propMu.RLock()
	a := i.aspect
	i.propMu.RUnlock()
	if i.Image == nil || a == 0 {
		if i.File != "" || i.Resource != nil {
			i.Refresh()
		}
	}
	return i.baseObject.MinSize()
}

// Move the image object to a new position, relative to its parent top, left corner.
func (i *Image) Move(pos geo.Position) {
	if i.Position() == pos {
		return
	}

	i.baseObject.Move(pos)

	repaint(i)
}

// Refresh causes this image to be redrawn with its configured state.
func (i *Image) Refresh() {
	// Snapshot source fields to avoid TOCTOU races with concurrent writers.
	i.propMu.RLock()
	file := i.File
	res := i.Resource
	img := i.Image
	i.propMu.RUnlock()

	rc, err := i.updateReader(file, res)
	if err != nil {
		bellui.LogError("Failed to load image", err)
		return
	}
	if rc != nil {
		rcMem := rc
		defer rcMem.Close()
	}

	if file != "" || res != nil || img != nil {
		r, err := i.updateAspectAndMinSize(rc)
		if err != nil {
			bellui.LogError("Failed to load image", err)
			return
		}
		rc = io.NopCloser(r)
	} else {
		i.propMu.RLock()
		prev := i.previousRender
		i.propMu.RUnlock()
		if prev {
			i.propMu.Lock()
			i.previousRender = false
			i.propMu.Unlock()

			Refresh(i)
			return
		}
		return
	}

	if file != "" || res != nil {
		size := i.Size()
		width := size.Width
		height := size.Height

		if width == 0 || height == 0 {
			return
		}

		i.propMu.RLock()
		isSVG := i.isSVG
		i.propMu.RUnlock()
		if isSVG {
			tex, err := i.renderSVG(width, height)
			if err != nil {
				bellui.LogError("Failed to render SVG", err)
				return
			}
			i.Image = tex
		} else {
			if rc == nil {
				return
			}

			decoded, _, err := image.Decode(rc)
			if err != nil {
				bellui.LogError("Failed to render image", err)
				return
			}
			i.Image = decoded
		}
	}

	i.propMu.Lock()
	i.previousRender = true
	i.propMu.Unlock()
	Refresh(i)
}

// Resize on an image will scale the content or reposition it according to FillMode.
// It will normally cause a Refresh to ensure the pixels are recalculated.
func (i *Image) Resize(s geo.Size) {
	if s == i.Size() {
		return
	}
	i.baseObject.Resize(s)
	if i.FillMode == ImageFillOriginal && i.Size().Height > 2 { // we can just ask for a GPU redraw to align
		Refresh(i)
		return
	}

	i.baseObject.Resize(s)
	i.propMu.RLock()
	isSVG := i.isSVG
	i.propMu.RUnlock()
	if isSVG || i.Image == nil {
		i.Refresh() // we need to rasterise at the new size
	} else {
		Refresh(i) // just re-size using GPU scaling
	}
}

// NewImageFromFile creates a new image from a local file.
// Images returned from this method will scale to fit the canvas object.
// The method for scaling can be set using the Fill field.
func NewImageFromFile(file string) *Image {
	return &Image{File: file}
}

// NewImageFromURI creates a new image from named resource.
// File URIs will read the file path and other schemes will download the data into a resource.
// HTTP and HTTPs URIs will use the GET method by default to request the resource.
// Images returned from this method will scale to fit the canvas object.
// The method for scaling can be set using the Fill field.
//
// Since: 1.0
func NewImageFromURI(uri bellui.URI) *Image {
	if uri.Scheme() == "file" && len(uri.String()) > 7 {
		return NewImageFromFile(uri.Path())
	}

	var read io.ReadCloser

	read, err := storage.Reader(uri) // attempt unknown / http file type
	if err != nil {
		bellui.LogError("Failed to open image URI", err)
		return &Image{}
	}

	defer read.Close()
	return NewImageFromReader(read, filepath.Base(uri.String()))
}

// NewImageFromReader creates a new image from a data stream.
// The name parameter is required to uniquely identify this image (for caching etc.).
// If the image in this io.Reader is an SVG, the name should end ".svg".
// Images returned from this method will scale to fit the canvas object.
// The method for scaling can be set using the Fill field.
//
// Since: 1.0
func NewImageFromReader(read io.Reader, name string) *Image {
	data, err := io.ReadAll(read)
	if err != nil {
		bellui.LogError("Unable to read image data", err)
		return nil
	}

	res := &bellui.StaticResource{
		StaticName:    name,
		StaticContent: data,
	}

	return NewImageFromResource(res)
}

// NewImageFromResource creates a new image by loading the specified resource.
// Images returned from this method will scale to fit the canvas object.
// The method for scaling can be set using the Fill field.
func NewImageFromResource(res bellui.Resource) *Image {
	return &Image{Resource: res}
}

// NewImageFromImage returns a new Image instance that is rendered from the Go
// image.Image passed in.
// Images returned from this method will scale to fit the canvas object.
// The method for scaling can be set using the Fill field.
func NewImageFromImage(img image.Image) *Image {
	return &Image{Image: img}
}

func (i *Image) name() string {
	if i.Resource != nil {
		return i.Resource.Name()
	} else if i.File != "" {
		return i.File
	}
	return ""
}

func (i *Image) updateReader(file string, res bellui.Resource) (io.ReadCloser, error) {
	i.propMu.Lock()
	i.isSVG = false
	i.propMu.Unlock()
	if res != nil {
		resSVG := svg.IsResourceSVG(res)
		i.propMu.Lock()
		i.isSVG = resSVG
		i.propMu.Unlock()

		// For non-SVG streaming resources, use Open() to avoid loading
		// all bytes into memory. SVGs need Content() for colorization.
		if sr, ok := res.(bellui.StreamingResource); ok && !resSVG {
			return sr.Open()
		}

		content := res.Content()
		if r, ok := res.(bellui.ThemedResource); resSVG && ok {
			th := cache.WidgetTheme(i)
			if th != nil {
				col := th.Color(r.ThemeColorName(), bellui.CurrentApp().Settings().ThemeVariant())
				var err error
				content, err = svg.Colorize(content, col)
				if err != nil {
					bellui.LogError("", err)
				}
			}
		}
		return io.NopCloser(bytes.NewReader(content)), nil
	} else if file != "" {
		fd, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		i.propMu.Lock()
		i.isSVG = svg.IsFileSVG(file)
		i.propMu.Unlock()
		return fd, nil
	}
	return nil, nil
}

func (i *Image) updateAspectAndMinSize(reader io.Reader) (io.Reader, error) {
	var pixWidth, pixHeight int

	if reader != nil {
		r, width, height, aspect, err := i.imageDetailsFromReader(reader)
		if err != nil {
			return nil, err
		}
		reader = r
		i.propMu.Lock()
		i.aspect = aspect
		i.propMu.Unlock()
		pixWidth, pixHeight = width, height
	} else if i.Image != nil {
		original := i.Image.Bounds().Size()
		i.propMu.Lock()
		i.aspect = float32(original.X) / float32(original.Y)
		i.propMu.Unlock()
		pixWidth, pixHeight = original.X, original.Y
	} else {
		return nil, errors.New("no matching image source")
	}

	if i.FillMode == ImageFillOriginal {
		i.SetMinSize(scale.ToBelluiSize(i, pixWidth, pixHeight))
	}
	return reader, nil
}

func (i *Image) imageDetailsFromReader(source io.Reader) (reader io.Reader, width, height int, aspect float32, err error) {
	if source == nil {
		return nil, 0, 0, 0, errors.New("no matching reading reader")
	}

	i.propMu.RLock()
	isSVG := i.isSVG
	i.propMu.RUnlock()
	if isSVG {
		var err error

		icon, err := svg.NewDecoder(source)
		if err != nil {
			return nil, 0, 0, 0, err
		}
		i.propMu.Lock()
		i.icon = icon
		i.propMu.Unlock()
		config := icon.Config()
		width, height = config.Width, config.Height
		aspect = config.Aspect
	} else {
		var buf bytes.Buffer
		tee := io.TeeReader(source, &buf)
		reader = io.MultiReader(&buf, source)

		config, _, err := image.DecodeConfig(tee)
		if err != nil {
			return nil, 0, 0, 0, err
		}
		width, height = config.Width, config.Height
		aspect = float32(width) / float32(height)
	}
	return reader, width, height, aspect, err
}

func (i *Image) renderSVG(width, height float32) (image.Image, error) {
	c := bellui.CurrentApp().Driver().CanvasForObject(i)
	screenWidth, screenHeight := int(width), int(height)
	if c != nil {
		// We want real output pixel count not just the screen coordinate space (i.e. macOS Retina)
		screenWidth, screenHeight = c.PixelCoordinateForPosition(geo.Position{X: width, Y: height})
	} else { // no canvas info, assume HiDPI
		screenWidth *= 2
		screenHeight *= 2
	}

	tex := cache.GetSvg(i.name(), i, screenWidth, screenHeight)
	if tex != nil {
		return tex, nil
	}

	i.propMu.RLock()
	icon := i.icon
	i.propMu.RUnlock()
	var err error
	tex, err = icon.Draw(screenWidth, screenHeight)
	if err != nil {
		return nil, err
	}
	cache.SetSvg(i.name(), i, tex, screenWidth, screenHeight)
	return tex, nil
}
