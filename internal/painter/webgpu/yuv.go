//go:build !gl

package webgpu

import (
	"encoding/binary"
	"fmt"
	"image"
	"math"

	"github.com/belluikit/webgpu/wgpu"

	"github.com/belluikit/bellui/internal/ycbcr"
)

// Y'CbCr video frames (#1107). A raster set with SetYCbCr gets a texture like
// any other, an RGBA one the simple shader draws, plus the planes behind it:
// each frame uploads its Y, Cb and Cr planes, one byte a sample, and a pass of
// the yuv shader converts them into the RGBA texture by the frame's own colour
// matrix and range. Nothing else in the painter knows the difference.

// yuvFormat is the format of a converted frame's texture. Plain unorm, not
// the surface's (possibly sRGB) format, so the shader's output is stored as
// the bytes an RGBA upload would have stored.
const yuvFormat = wgpu.TextureFormatRGBA8Unorm

// yuvUniformSize is the yuv shader's uniform block: three vec4 rows.
const yuvUniformSize = 48

// yuvPlanes are the plane textures and per-frame state behind a converted
// frame's texture.
type yuvPlanes struct {
	y, cb, cr *Texture
	uniform   *wgpu.Buffer
	bind      *wgpu.BindGroup
	target    *wgpu.TextureView // the RGBA texture's view, the pass's attachment
}

func (yp *yuvPlanes) release() {
	if yp.bind != nil {
		yp.bind.Release()
		yp.bind = nil
	}
	if yp.uniform != nil {
		destroyBuffer(yp.uniform)
		yp.uniform = nil
	}
	for _, t := range []*Texture{yp.y, yp.cb, yp.cr} {
		if t != nil {
			t.Release()
		}
	}
	yp.y, yp.cb, yp.cr = nil, nil, nil
}

// FitsYCbCr reports whether t was made by CreateYCbCrTexture for frames of
// img's size and layout, so WriteYCbCr can convert img into it.
func (t *Texture) FitsYCbCr(img *image.YCbCr) bool {
	if t == nil || t.yuv == nil || t.yuv.y == nil {
		return false
	}
	cw, ch := ycbcr.ChromaSize(img)
	return t.w == img.Rect.Dx() && t.h == img.Rect.Dy() && t.yuv.cb.w == cw && t.yuv.cb.h == ch
}

// CreateYCbCrTexture creates an RGBA texture for frames of img's size and
// layout, with the plane textures and bind group WriteYCbCr converts through.
// It does not convert img: call WriteYCbCr for that.
func (d *Device) CreateYCbCrTexture(img *image.YCbCr) (*Texture, error) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	cw, ch := ycbcr.ChromaSize(img)
	if w <= 0 || h <= 0 || cw <= 0 || ch <= 0 {
		return nil, fmt.Errorf("empty Y'CbCr frame %dx%d", w, h)
	}
	if err := d.ensureYUVPipeline(); err != nil {
		return nil, err
	}

	tex, err := d.newTexture("yuv-rgba", w, h, yuvFormat, wgpu.TextureUsageTextureBinding|wgpu.TextureUsageRenderAttachment)
	if err != nil {
		return nil, err
	}
	yp := &yuvPlanes{target: tex.view}
	tex.yuv = yp
	fail := func(err error) (*Texture, error) {
		tex.Release()
		return nil, err
	}
	if yp.y, err = d.newTexture("yuv-y", w, h, wgpu.TextureFormatR8Unorm, wgpu.TextureUsageTextureBinding|wgpu.TextureUsageCopyDst); err != nil {
		return fail(err)
	}
	if yp.cb, err = d.newTexture("yuv-cb", cw, ch, wgpu.TextureFormatR8Unorm, wgpu.TextureUsageTextureBinding|wgpu.TextureUsageCopyDst); err != nil {
		return fail(err)
	}
	if yp.cr, err = d.newTexture("yuv-cr", cw, ch, wgpu.TextureFormatR8Unorm, wgpu.TextureUsageTextureBinding|wgpu.TextureUsageCopyDst); err != nil {
		return fail(err)
	}
	if yp.uniform, err = d.device.TryCreateBuffer(&wgpu.BufferDescriptor{
		Label: "yuv-uniform",
		Size:  yuvUniformSize,
		Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst,
	}); err != nil {
		return fail(err)
	}
	if yp.bind, err = d.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: d.bglYUV,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: yp.uniform, Size: yuvUniformSize},
			{Binding: 1, TextureView: yp.y.view},
			{Binding: 2, TextureView: yp.cb.view},
			{Binding: 3, TextureView: yp.cr.view},
			{Binding: 4, Sampler: d.sampler},
		},
	}); err != nil {
		return fail(err)
	}
	return tex, nil
}

// WriteYCbCr uploads img's planes into t, which CreateYCbCrTexture made for
// frames like it, and converts them into t's RGBA by the given matrix and
// range. The conversion is submitted on its own, ahead of the frame being
// recorded, so it does not break the frame's render pass.
func (d *Device) WriteYCbCr(t *Texture, img *image.YCbCr, matrix int, fullRange bool) error {
	if !t.FitsYCbCr(img) {
		return fmt.Errorf("texture does not fit a %v %v frame", img.Rect, img.SubsampleRatio)
	}
	if d == nil || d.queue == nil {
		return fmt.Errorf("no device queue to write with")
	}
	yp := t.yuv
	o := img.Rect.Min
	d.writePlane(yp.y, img.Y[img.YOffset(o.X, o.Y):], img.YStride)
	ci := img.COffset(o.X, o.Y)
	d.writePlane(yp.cb, img.Cb[ci:], img.CStride)
	d.writePlane(yp.cr, img.Cr[ci:], img.CStride)
	d.queue.WriteBuffer(yp.uniform, 0, yuvUniformBytes(ycbcr.Coefficients(matrix, fullRange)))

	enc, err := d.device.TryCreateCommandEncoder(nil)
	if err != nil {
		return err
	}
	defer enc.Release()
	pass := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View: yp.target, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore,
		}},
	})
	pass.SetPipeline(d.yuvPipe)
	pass.SetBindGroup(0, yp.bind, nil)
	pass.Draw(3, 1, 0, 0)
	err = pass.TryEnd()
	pass.Release()
	if err != nil {
		return err
	}
	cmd, err := enc.TryFinish(nil)
	if err != nil {
		return err
	}
	d.queue.Submit(cmd)
	cmd.Release()
	return nil
}

// writePlane uploads one plane, read from pix at stride bytes a row.
func (d *Device) writePlane(t *Texture, pix []byte, stride int) {
	extent := wgpu.Extent3D{Width: uint32(t.w), Height: uint32(t.h), DepthOrArrayLayers: 1}
	d.queue.WriteTexture(t.tex.AsImageCopy(), pix[:stride*(t.h-1)+t.w], &wgpu.TexelCopyBufferLayout{
		BytesPerRow:  uint32(stride),
		RowsPerImage: uint32(t.h),
	}, &extent)
}

// yuvUniformBytes lays the coefficient rows out as the yuv shader's uniform
// block.
func yuvUniformBytes(k [3][4]float32) []byte {
	b := make([]byte, yuvUniformSize)
	for i, row := range k {
		for j, v := range row {
			binary.LittleEndian.PutUint32(b[(i*4+j)*4:], math.Float32bits(v))
		}
	}
	return b
}

// newTexture creates a w×h texture and its view.
func (d *Device) newTexture(label string, w, h int, format wgpu.TextureFormat, usage wgpu.TextureUsage) (*Texture, error) {
	tex, err := d.device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         label,
		Size:          wgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        format,
		Usage:         usage,
	})
	if err != nil {
		return nil, err
	}
	view, err := tex.TryCreateView(nil)
	if err != nil {
		destroyTexture(tex)
		return nil, err
	}
	return &Texture{tex: tex, view: view, w: w, h: h}, nil
}

// ensureYUVPipeline builds the yuv shader's bind-group layout and pipeline on
// first use. They are the device's, released by Destroy.
func (d *Device) ensureYUVPipeline() error {
	if d.yuvPipe != nil {
		return nil
	}
	plane := wgpu.TextureBindingLayout{SampleType: wgpu.TextureSampleTypeFloat, ViewDimension: wgpu.TextureViewDimension2D}
	bgl, err := d.device.TryCreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "yuv",
		Entries: []wgpu.BindGroupLayoutEntry{
			{Binding: 0, Visibility: wgpu.ShaderStageFragment, Buffer: wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeUniform}},
			{Binding: 1, Visibility: wgpu.ShaderStageFragment, Texture: plane},
			{Binding: 2, Visibility: wgpu.ShaderStageFragment, Texture: plane},
			{Binding: 3, Visibility: wgpu.ShaderStageFragment, Texture: plane},
			{Binding: 4, Visibility: wgpu.ShaderStageFragment, Sampler: wgpu.SamplerBindingLayout{Type: wgpu.SamplerBindingTypeFiltering}},
		},
	})
	if err != nil {
		return fmt.Errorf("yuv bind-group layout: %w", err)
	}
	pl, err := d.device.TryCreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "pl-yuv",
		BindGroupLayouts: []*wgpu.BindGroupLayout{bgl},
	})
	if err != nil {
		bgl.Release()
		return fmt.Errorf("yuv pipeline layout: %w", err)
	}
	module, err := d.shaderModule("yuv")
	if err != nil {
		pl.Release()
		bgl.Release()
		return err
	}
	pipe, err := d.device.TryCreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  "yuv",
		Layout: pl,
		Vertex: wgpu.VertexState{Module: module, EntryPoint: VertexEntry},
		Fragment: &wgpu.FragmentState{
			Module:     module,
			EntryPoint: FragmentEntry,
			Targets:    []wgpu.ColorTargetState{{Format: yuvFormat, WriteMask: wgpu.ColorWriteMaskAll}},
		},
		Primitive:   wgpu.PrimitiveState{Topology: wgpu.PrimitiveTopologyTriangleList, CullMode: wgpu.CullModeNone},
		Multisample: wgpu.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
	})
	if err != nil {
		pl.Release()
		bgl.Release()
		return fmt.Errorf("yuv pipeline: %w", err)
	}
	d.bglYUV, d.plYUV, d.yuvPipe = bgl, pl, pipe
	return nil
}

// releaseYUVPipeline frees what ensureYUVPipeline built.
func (d *Device) releaseYUVPipeline() {
	if d.yuvPipe != nil {
		d.yuvPipe.Release()
		d.yuvPipe = nil
	}
	if d.plYUV != nil {
		d.plYUV.Release()
		d.plYUV = nil
	}
	if d.bglYUV != nil {
		d.bglYUV.Release()
		d.bglYUV = nil
	}
}
