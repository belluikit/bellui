package webgpu

// StencilMode selects the depth-stencil state baked into a pipeline (stencil
// clipping + path fill, pre-rebrand tracker #202/#222). Every pipeline carries one so it validates
// against the depth-stencil attachment now present on every render pass; the
// default StencilNone disables stencil entirely (compare Always, no write) and
// writes color, matching the pre-stencil behaviour. The painter switches mode
// via the pipeline key while writing a clip mask or filling a path.
type StencilMode int

const (
	// StencilNone disables the stencil test and writes color (the default for
	// every normal draw): stencil compare Always, no stencil write.
	StencilNone StencilMode = iota
	// StencilWriteReplace writes the stencil reference to every bit where
	// fragments pass and masks color off: with ref 0 it zeroes the stencil,
	// emulating a mid-pass clear.
	StencilWriteReplace
	// StencilClearWinding zeroes the winding bits (paint.StencilWindingMask)
	// and keeps the clip depth, color masked off: a path op's fresh start
	// inside the rounded clips around it (#1015).
	StencilClearWinding
	// StencilClearDepth zeroes the depth bits and keeps the winding bits,
	// color masked off: a first rounded clip on a stencil not known to be
	// zero, inside a path clip as well.
	StencilClearDepth
	// StencilWriteIncrDecr increments the winding bits on front faces and
	// decrements them on back faces (non-zero winding), color masked off: the
	// path clip / fill winding pass outside every rounded clip.
	StencilWriteIncrDecr
	// StencilWriteIncrDecrInClip is StencilWriteIncrDecr only where the depth
	// bits equal the reference: the winding pass inside a rounded clip, which
	// so cuts the path (#1015).
	StencilWriteIncrDecrInClip
	// StencilWriteInvert inverts the winding bits, color masked off: the
	// even-odd fill winding pass.
	StencilWriteInvert
	// StencilWriteInvertInClip is StencilWriteInvert inside a rounded clip.
	StencilWriteInvertInClip
	// StencilTestEqual draws color only where the depth bits equal the
	// reference (rounded-rect clip content).
	StencilTestEqual
	// StencilTestNotEqual draws color only where the winding bits are not the
	// reference, 0 (path clip content and the path-fill bounding-box quad).
	StencilTestNotEqual
	// StencilStampBit0 to StencilStampBit3 stamp and erase a rounded clip's
	// depth: where the depth bits, all but bit n, equal the reference, they
	// are replaced with it, color masked off. Depth d stamps with bit
	// paint.ClipStampBit(d) and reference paint.ClipDepthCode(d), and erases
	// with paint.ClipDepthCode(d-1), so a nested clip only narrows the clip
	// around it (#1012, #1015).
	StencilStampBit0
	StencilStampBit1
	StencilStampBit2
	StencilStampBit3
	// StencilStampWinding replaces the depth bits with the reference where the
	// winding bits are non-zero, color masked off: a path clip becomes a
	// clip level, so what is drawn inside it, rounded clips included, is cut
	// by it (#1015).
	StencilStampWinding
)

// PipelineKey identifies a unique render pipeline (PSO). A pipeline is keyed by
// the shader module, the fragment entry point (so blur's fs_main_h / fs_main_v
// get distinct pipelines), the blend mode, the depth-stencil mode, whether
// hardware depth testing is enabled (Scene3D z-buffering, pre-rebrand tracker #224), and the
// render-target texture format (kept as a neutral uint32 so this file stays free
// of GPU binding types).
type PipelineKey struct {
	Shader   string
	Fragment string
	Blend    BlendMode
	Stencil  StencilMode
	// Depth enables hardware z-buffering (depth write + less-than test) on top of
	// the always-present depth-stencil attachment. Only the Scene3D pipeline sets
	// it; every 2D op leaves it false (depth inert, compare Always, no write).
	Depth  bool
	Format uint32
}

// PipelineCache lazily builds and memoizes pipelines by key. It is generic over
// the pipeline handle type T so the device-independent caching logic is
// testable without the WebGPU binding; the //go:build webgpu painter
// instantiates it with *wgpu.RenderPipeline.
type PipelineCache[T any] struct {
	build func(PipelineKey) (T, error)
	items map[PipelineKey]T
}

// NewPipelineCache returns a cache that calls build once per distinct key.
func NewPipelineCache[T any](build func(PipelineKey) (T, error)) *PipelineCache[T] {
	return &PipelineCache[T]{build: build, items: map[PipelineKey]T{}}
}

// Get returns the pipeline for the key, building (and caching) it on first use.
func (c *PipelineCache[T]) Get(key PipelineKey) (T, error) {
	if p, ok := c.items[key]; ok {
		return p, nil
	}
	p, err := c.build(key)
	if err != nil {
		var zero T
		return zero, err
	}
	c.items[key] = p
	return p, nil
}

// Len returns the number of cached pipelines.
func (c *PipelineCache[T]) Len() int { return len(c.items) }

// Release calls fn for every cached pipeline and empties the cache. Used to
// free GPU resources (e.g. on a scale/format change or painter teardown).
func (c *PipelineCache[T]) Release(fn func(T)) {
	for k, p := range c.items {
		if fn != nil {
			fn(p)
		}
		delete(c.items, k)
	}
}
