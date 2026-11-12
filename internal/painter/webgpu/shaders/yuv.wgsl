// yuv — converts a Y'CbCr video frame to RGBA, in a pass of its own into the
// raster's texture, which the simple shader then draws like any other (#1107).
// One triangle covers the target, built from the vertex index: no vertex
// buffer.

struct Uniforms {
    // Each row maps (y, cb, cr, 1), samples in [0, 1], to one channel.
    r: vec4<f32>,  // offset 0
    g: vec4<f32>,  // offset 16
    b: vec4<f32>,  // offset 32
};

@group(0) @binding(0) var<uniform> u: Uniforms;
@group(0) @binding(1) var tex_y: texture_2d<f32>;
@group(0) @binding(2) var tex_cb: texture_2d<f32>;
@group(0) @binding(3) var tex_cr: texture_2d<f32>;
@group(0) @binding(4) var samp: sampler;

struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
    @location(0) uv: vec2<f32>,
};

@vertex
fn vs_main(@builtin(vertex_index) i: u32) -> VertexOutput {
    // uv (0,0), (2,0), (0,2): the triangle's inside covers [0,1]², with v=0
    // the texture's top row, the first row the frame uploads.
    let uv = vec2<f32>(f32((i << 1u) & 2u), f32(i & 2u));
    var out: VertexOutput;
    out.pos = vec4<f32>(uv.x * 2.0 - 1.0, 1.0 - uv.y * 2.0, 0.0, 1.0);
    out.uv = uv;
    return out;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    let s = vec4<f32>(
        textureSample(tex_y, samp, in.uv).r,
        textureSample(tex_cb, samp, in.uv).r,
        textureSample(tex_cr, samp, in.uv).r,
        1.0,
    );
    let rgb = vec3<f32>(dot(u.r, s), dot(u.g, s), dot(u.b, s));
    return vec4<f32>(clamp(rgb, vec3<f32>(0.0), vec3<f32>(1.0)), 1.0);
}
