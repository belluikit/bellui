package webgpu

// StencilMode selects the depth-stencil state baked into a pipeline (stencil
// clipping + path fill, #202/#222). Every pipeline carries one so it validates
// against the depth-stencil attachment now present on every render pass; the
// default StencilNone disables stencil entirely (compare Always, no write) and
// writes color, matching the pre-stencil behaviour. The painter switches mode
// via the pipeline key while writing a clip mask or filling a path.
type StencilMode int

const (
	// StencilNone disables the stencil test and writes color (the default for
	// every normal draw): stencil compare Always, no stencil write.
	StencilNone StencilMode = iota
	// StencilWriteReplace writes the stencil reference where fragments pass and
	// masks color off — used to stamp a rounded-rect clip mask (ref=1) and to
	// zero the stencil (ref=0) emulating a mid-pass clear.
	StencilWriteReplace
	// StencilWriteIncrDecr increments the stencil on front faces and decrements
	// on back faces (non-zero winding), color masked off — the path clip / fill
	// winding pass.
	StencilWriteIncrDecr
	// StencilWriteInvert inverts the stencil bits, color masked off — the
	// even-odd fill winding pass.
	StencilWriteInvert
	// StencilTestEqual draws color only where stencil == reference (rounded-rect
	// clip content).
	StencilTestEqual
	// StencilTestNotEqual draws color only where stencil != reference (path clip
	// content and the path-fill bounding-box quad; reference is 0).
	StencilTestNotEqual
)

// PipelineKey identifies a unique render pipeline (PSO). A pipeline is keyed by
// the shader module, the fragment entry point (so blur's fs_main_h / fs_main_v
// get distinct pipelines), the blend mode, the depth-stencil mode, whether
// hardware depth testing is enabled (Scene3D z-buffering, #224), and the
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
