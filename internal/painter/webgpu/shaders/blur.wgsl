// blur — separable 9-tap Gaussian. Port of blur.vert + blur_h.frag/blur_v.frag.
// The horizontal and vertical passes share the vertex stage and differ only in
// the sampling axis, exposed as fs_main_h and fs_main_v.

struct Uniforms {
    resolution: vec2<f32>,  // offset 0
    blur_radius: f32,       // offset 8
};

@group(0) @binding(0) var<uniform> u: Uniforms;
@group(0) @binding(1) var tex: texture_2d<f32>;
@group(0) @binding(2) var samp: sampler;

struct VertexInput {
    @location(0) vert: vec3<f32>,
    @location(1) tex_coord: vec2<f32>,
};

struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
    @location(0) frag_tex_coord: vec2<f32>,
};

@vertex
fn vs_main(in: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    out.pos = vec4<f32>(in.vert, 1.0);
    out.frag_tex_coord = in.tex_coord;
    return out;
}

// 9-tap Gaussian kernel (sigma ~= blur_radius/3), shared by both passes.
fn blur_along(uv: vec2<f32>, axis: vec2<f32>) -> vec4<f32> {
    let step = axis * u.blur_radius;
    var sum = vec4<f32>(0.0);
    sum = sum + textureSample(tex, samp, uv + -4.0 * step) * 0.0162;
    sum = sum + textureSample(tex, samp, uv + -3.0 * step) * 0.0540;
    sum = sum + textureSample(tex, samp, uv + -2.0 * step) * 0.1216;
    sum = sum + textureSample(tex, samp, uv + -1.0 * step) * 0.1945;
    sum = sum + textureSample(tex, samp, uv) * 0.2270;
    sum = sum + textureSample(tex, samp, uv + 1.0 * step) * 0.1945;
    sum = sum + textureSample(tex, samp, uv + 2.0 * step) * 0.1216;
    sum = sum + textureSample(tex, samp, uv + 3.0 * step) * 0.0540;
    sum = sum + textureSample(tex, samp, uv + 4.0 * step) * 0.0162;
    return sum;
}

@fragment
fn fs_main_h(in: VertexOutput) -> @location(0) vec4<f32> {
    return blur_along(in.frag_tex_coord, vec2<f32>(1.0 / u.resolution.x, 0.0));
}

@fragment
fn fs_main_v(in: VertexOutput) -> @location(0) vec4<f32> {
    return blur_along(in.frag_tex_coord, vec2<f32>(0.0, 1.0 / u.resolution.y));
}
