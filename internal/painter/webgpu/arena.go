package webgpu

import "slices"

// Per-frame vertex/uniform arena bookkeeping (pre-rebrand tracker #255).
//
// The slot pool (pre-rebrand tracker #245) reuses a vertex buffer, a uniform buffer and a bind group
// per draw across frames, but still issues two queue.WriteBuffer calls per draw
// (vertex + uniform), so a frame with N draws makes ~2N WriteBuffer cgo calls.
// The arena instead accumulates every draw's vertex data into one CPU staging
// slice and every draw's uniform block into another, then the //go:build webgpu
// Device flushes each with a single WriteBuffer per frame — collapsing ~2N calls
// to ~2. Draws index the arenas: vertices via a byte offset on SetVertexBuffer,
// uniforms via a dynamic bind-group offset.
//
// This file is the device-independent half: pure CPU slice accumulation with no
// GPU binding types, so it compiles everywhere and is unit-tested in normal CI
// (arena_test.go). The Device owns the matching GPU buffers and the flush.

// uniformBlockStride is the per-block stride in the uniform arena. It is the
// WebGPU minUniformBufferOffsetAlignment (256) so each block's offset is a valid
// dynamic offset, and it also exceeds every uniform struct in uniforms.go (the
// largest, RoundRectangleUniforms, is 144 bytes) so one stride fits them all and
// a single 256-byte bind-group window addresses any block.
const uniformBlockStride = 256

// arena accumulates one frame's vertex bytes and uniform blocks into two CPU
// staging buffers. The byte offset it returns for a vertex chunk is always
// 4-aligned (every chunk is a whole number of float32s) and the dynamic offset
// it returns for a uniform block is always a multiple of uniformBlockStride.
type arena struct {
	vtx []byte
	uni []byte
}

// reset truncates both staging buffers for a new frame, keeping their capacity.
func (a *arena) reset() {
	a.vtx = a.vtx[:0]
	a.uni = a.uni[:0]
}

// addVertices appends v and returns its byte offset within the vertex arena.
// The offset is 4-aligned for SetVertexBuffer because every chunk is float32 data.
func (a *arena) addVertices(v []byte) int {
	off := len(a.vtx)
	a.vtx = append(a.vtx, v...)
	return off
}

// addUniform appends u padded up to uniformBlockStride and returns its dynamic
// offset (a multiple of the stride). u must be no larger than uniformBlockStride.
func (a *arena) addUniform(u []byte) int {
	off := len(a.uni)
	// Grown and cleared rather than appended from make([]byte, pad), which
	// allocates the padding on every call in a -race build (#447).
	a.uni = slices.Grow(a.uni, uniformBlockStride)[:off+uniformBlockStride]
	clear(a.uni[off+copy(a.uni[off:], u):])
	return off
}

// vtxLen / uniLen report the bytes accumulated so far this frame (the amount a
// single WriteBuffer flushes, and the value the Device tracks as a high-water
// mark to pre-size the GPU arenas before the next frame).
func (a *arena) vtxLen() int { return len(a.vtx) }
func (a *arena) uniLen() int { return len(a.uni) }
