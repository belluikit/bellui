//go:build !gl && !wasm

// (!wasm: acquires a real WebGPU device, see arena_readback_test.go.)

package webgpu

import (
	"strings"
	"testing"

	"github.com/belluikit/webgpu/wgpu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clipsAway reads each shader's output position straight from its vertices.
// This fails when a shader's layout or vertex stage stops matching what it
// reads, which would cull draws that still paint.
func TestClipPositionsMatchShaders(t *testing.T) {
	for name, cp := range clipPositions {
		desc, ok := DescriptorFor(name)
		require.True(t, ok, name)
		assert.Equal(t, desc.Vertex.Stride/4, cp.stride, "%s: vertex stride in floats", name)
		assert.Equal(t, 0, desc.Vertex.Attrs[0].Offset, "%s: position comes first", name)
		src := string(ShaderSource(name))
		if cp.offset == 0 {
			assert.Contains(t, src, "out.pos = vec4<f32>(in.vert,", "%s: position is the vertex", name)
			continue
		}
		assert.Equal(t, cp.offset*4, desc.Vertex.Attrs[1].Offset, "%s: normal offset", name)
		assert.Contains(t, src, "out.pos = vec4<f32>(in.vert + in.normal,", "%s: position is vertex plus normal", name)
	}
	for _, name := range []string{"line", "scene3d"} {
		_, ok := clipPositions[name]
		assert.False(t, ok, "%s takes its position from a uniform; it must not be culled", name)
		assert.False(t, strings.Contains(string(ShaderSource(name)), "out.pos = vec4<f32>(in.vert + in.normal,"))
	}
}

// TestClipsAway checks which draws a scissored pass skips: only one whose every
// vertex lies on the far side of a clip edge, by more than the pixel of slack.
func TestClipsAway(t *testing.T) {
	instance := wgpu.CreateInstance(chooseInstanceBackends())
	defer instance.Release()
	adapter, err := instance.RequestAdapter(nil)
	if err != nil {
		noGPU(t, "no WebGPU adapter available: %v", err)
	}
	defer adapter.Release()
	device, err := adapter.RequestDevice(nil)
	if err != nil {
		noGPU(t, "no WebGPU device available: %v", err)
	}

	// d owns device and queue from here: Destroy releases them.
	d := &Device{
		device:        device,
		queue:         device.GetQueue(),
		depthStencils: map[[2]int]*depthStencil{},
		config:        &wgpu.SurfaceConfiguration{Format: wgpu.TextureFormatRGBA8Unorm},
	}
	defer d.Destroy()
	root, err := d.CreateRenderTexture(400, 300)
	require.NoError(t, err)
	defer root.Release()
	d.encoder, err = device.TryCreateCommandEncoder(nil)
	require.NoError(t, err)
	d.targets = append(d.targets[:0], renderTarget{view: root.View(), tex: root.tex, w: 400, h: 300})
	d.beginTopPass()

	// quad returns a rectangle-shader draw covering the target pixels
	// (x0,y0)-(x1,y1), split between vertex and normal as the painter does.
	quad := func(shader string, x0, y0, x1, y1 float32) DrawCall {
		nx := func(px float32) float32 { return px/400*2 - 1 }
		ny := func(py float32) float32 { return 1 - py/300*2 }
		var v []float32
		for _, p := range [][2]float32{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
			v = append(v, 0, 0, nx(p[0]), ny(p[1]))
		}
		return DrawCall{Key: PipelineKey{Shader: shader}, Vertices: v, VertexCount: 4}
	}

	// With no scissor the pass clips to its target.
	assert.False(t, d.clipsAway(quad("rectangle", 380, 280, 600, 600)), "partly on the target")
	assert.True(t, d.clipsAway(quad("rectangle", 500, 500, 600, 600)), "off the target")

	d.SetScissor(100, 100, 100, 50) // pixels (100,100)-(200,150)
	assert.False(t, d.clipsAway(quad("rectangle", 120, 110, 140, 130)), "inside")
	assert.False(t, d.clipsAway(quad("rectangle", 50, 50, 101, 101)), "overlapping a corner")
	assert.False(t, d.clipsAway(quad("rectangle", 201, 110, 220, 130)), "within the pixel of slack")
	assert.True(t, d.clipsAway(quad("rectangle", 210, 110, 230, 130)), "right of the clip")
	assert.True(t, d.clipsAway(quad("rectangle", 120, 160, 140, 400)), "below the clip")
	assert.True(t, d.clipsAway(quad("rectangle", 120, -50, 140, 90)), "above the clip")
	assert.True(t, d.clipsAway(quad("rectangle", 0, 110, 90, 130)), "left of the clip")
	assert.False(t, d.clipsAway(quad("line", 210, 110, 230, 130)), "line widens by a uniform and is never culled")

	d.endPass()
	cmd, err := d.encoder.TryFinish(nil)
	require.NoError(t, err)
	cmd.Release()
	d.encoder.Release()
	d.encoder = nil
	for _, ds := range d.frameDepths {
		ds.Release()
	}
	d.frameDepths = nil
}
