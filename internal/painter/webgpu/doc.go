// Package webgpu will host the WebGPU implementation of the backend-neutral
// gpu.Painter interface (WebGPU migration, pre-rebrand issue #198, Phase 2 / pre-rebrand tracker #201).
//
// At this stage the package only carries the WGSL shader ports (pre-rebrand tracker #205); the
// concrete painter is added in Phase 2. The shaders are embedded so the
// eventual painter can create shader modules without touching the filesystem,
// mirroring internal/painter/gl/shaders.go.
//
// # Shader porting conventions
//
// Each GLSL desktop/_es pair (internal/painter/gl/shaders) collapses to a
// single WGSL module in shaders/, containing both the vertex and fragment
// entry points. Entry points follow a fixed naming convention so the painter
// can look them up without per-shader configuration:
//
//   - vertex:   vs_main
//   - fragment: fs_main   (blur additionally exposes fs_main_h / fs_main_v)
//
// Coordinate systems differ between OpenGL and WebGPU; the ports preserve the
// GL fragment math exactly by translating at the boundary:
//
//   - Clip space: both APIs use x,y in [-1,1] with +y up, so the NDC vertex
//     positions are fed unchanged. (WebGPU's z range is [0,1] vs GL's [-1,1],
//     but every 2D shader here writes z=0, which is valid in both.)
//
//   - Framebuffer space: GL's gl_FragCoord has its origin bottom-left (y-up)
//     while WebGPU's @builtin(position) has its origin top-left (y-down). The
//     SDF shaders that sample the framebuffer position reconstruct the GL
//     convention as vec2(pos.x, frame_size.y - pos.y) before applying
//     inv_transform, so the SDF body stays byte-for-byte identical to the
//     GLSL. frame_size is in framebuffer pixels (== gl_FragCoord's range), so
//     frame_size.y is the correct flip pivot.
//
//   - inv_transform is uploaded by the painter exactly as in GL (it already
//     maps GL pixel space back to untransformed pixel space; see
//     gl.painter.uploadInvTransform).
//
//   - Texture V axis: GL samples with v=0 at the bottom, WebGPU with v=0 at
//     the top. This is a texture-upload/sampling concern handled by the
//     painter, not the shader — the WGSL samples texCoord as given.
//
// # Uniform buffer layout
//
// WGSL uniform buffers follow std140-like alignment (f32:4, vec2:8, vec3/vec4:
// 16, mat3x3:48 with 16-byte column stride, mat4x4:64). Each module's uniform
// struct is declared field-by-field with its byte offset noted so the painter
// can populate the buffer with a matching memory layout.
package webgpu
