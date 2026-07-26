// line — anti-aliased line with feathered edge. Port of line.vert/line.frag.

struct Uniforms {
    color: vec4<f32>,  // offset 0
    line_width: f32,   // offset 16
    feather: f32,      // offset 20
};

@group(0) @binding(0) var<uniform> u: Uniforms;

struct VertexInput {
    @location(0) vert: vec2<f32>,
    @location(1) normal: vec2<f32>,
};

struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
    @location(0) delta: vec2<f32>,
};

@vertex
fn vs_main(in: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    let delta = in.normal * u.line_width;
    out.delta = delta;
    out.pos = vec4<f32>(in.vert + delta, 0.0, 1.0);
    return out;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    let distance = length(in.delta);

    if (u.feather == 0.0 || distance <= u.line_width - u.feather) {
        return u.color;
    }
    let a = mix(u.color.a, 0.0, (distance - (u.line_width - u.feather)) / u.feather);
    return vec4<f32>(u.color.rgb, a);
}
