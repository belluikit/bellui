// scene3d — Phong-lit 3D mesh. Port of scene3d.vert/scene3d.frag.

struct Uniforms {
    mvp: mat4x4<f32>,         // offset 0
    light_dir: vec3<f32>,    // offset 64
    light_color: vec3<f32>,  // offset 80
    ambient: f32,            // offset 92
};

@group(0) @binding(0) var<uniform> u: Uniforms;

struct VertexInput {
    @location(0) vert: vec3<f32>,
    @location(1) normal: vec3<f32>,
    @location(2) color: vec4<f32>,
};

struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
    @location(0) v_normal: vec3<f32>,
    @location(1) v_color: vec4<f32>,
};

@vertex
fn vs_main(in: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    out.pos = u.mvp * vec4<f32>(in.vert, 1.0);
    out.v_normal = in.normal;
    out.v_color = in.color;
    return out;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    let n = normalize(in.v_normal);
    let diffuse = max(dot(n, -u.light_dir), 0.0);
    let lighting = u.light_color * (u.ambient + (1.0 - u.ambient) * diffuse);
    return vec4<f32>(in.v_color.rgb * lighting, in.v_color.a);
}
