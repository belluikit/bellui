// path — filled path geometry with per-vertex coverage. Port of path.vert/path.frag.

struct Uniforms {
    fill_color: vec4<f32>,  // offset 0
};

@group(0) @binding(0) var<uniform> u: Uniforms;

struct VertexInput {
    @location(0) vert: vec2<f32>,
    @location(1) alpha: f32,
};

struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
    @location(0) v_alpha: f32,
};

@vertex
fn vs_main(in: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    out.pos = vec4<f32>(in.vert, 0.0, 1.0);
    out.v_alpha = in.alpha;
    return out;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    return vec4<f32>(u.fill_color.rgb, u.fill_color.a * in.v_alpha);
}
