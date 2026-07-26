// simple — textured quad (images and text). Port of simple.vert/simple.frag.

struct Uniforms {
    alpha: f32,          // offset 0
    corner_radius: f32,  // offset 4  (pixels)
    size: vec2<f32>,     // offset 8  (pixels: size of the rendered image quad)
    inset: vec4<f32>,    // offset 16 (texcoord insets: minX, minY, maxX, maxY)
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

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    var alpha: f32 = 1.0;
    if (u.corner_radius > 0.5) {
        // normalize texture coords from [insetMin, insetMax] to [0, 1] so the
        // rounding works for cropped/covered images
        let normalized_coord = (in.frag_tex_coord - u.inset.xy) / (u.inset.zw - u.inset.xy);

        let p = normalized_coord * u.size;
        let half_size = u.size * 0.5;
        let dist = length(max(abs(p - half_size) - half_size + u.corner_radius, vec2<f32>(0.0))) - u.corner_radius;
        alpha = 1.0 - smoothstep(-1.0, 1.0, dist);
    }

    var tex_color = textureSample(tex, samp, in.frag_tex_coord);
    tex_color.a = tex_color.a * u.alpha * alpha;
    let rgb = tex_color.rgb * u.alpha * alpha;
    tex_color = vec4<f32>(rgb, tex_color.a);

    if (tex_color.a < 0.01) {
        discard;
    }
    return tex_color;
}
