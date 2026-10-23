// quad — instanced rectangles, rounded rectangles and atlas glyphs (#475).
//
// One instance is one shape. The vertex shader makes the instance's quad from
// the vertex index (a 4-vertex triangle strip), so a batch of shapes is one
// draw. Rectangles are in framebuffer pixels with a top-left origin.
//
// kind 0: a rounded rectangle, filled and stroked, by distance to its edges
// (the round_rectangle shader's maths; radius 0 is a plain rectangle).
// kind 1: a glyph: the atlas's coverage at uv, in fill_color.
// kind 2: a plain rectangle, hard-edged as the rectangle shader draws it: the
// fill inside, the stroke over the last stroke_width (params.x) pixels.

struct Uniforms {
    frame_size: vec2<f32>, // target size in pixels
};

@group(0) @binding(0) var<uniform> u: Uniforms;
@group(0) @binding(1) var atlas: texture_2d<f32>;
@group(0) @binding(2) var samp: sampler;

struct Instance {
    @location(0) rect: vec4<f32>,         // x1, y1, x2, y2
    @location(1) radius: vec4<f32>,       // tr, br, tl, bl
    @location(2) fill_color: vec4<f32>,
    @location(3) stroke_color: vec4<f32>,
    @location(4) params: vec4<f32>,       // stroke_width_half, edge_softness, kind, -
    @location(5) uv: vec4<f32>,           // u1, v1, u2, v2 (glyphs)
};

struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
    @location(0) @interpolate(flat) rect: vec4<f32>,
    @location(1) @interpolate(flat) radius: vec4<f32>,
    @location(2) @interpolate(flat) fill_color: vec4<f32>,
    @location(3) @interpolate(flat) stroke_color: vec4<f32>,
    @location(4) @interpolate(flat) params: vec4<f32>,
    @location(5) tex: vec2<f32>,
};

@vertex
fn vs_main(@builtin(vertex_index) vi: u32, inst: Instance) -> VertexOutput {
    // Strip order: top-left, bottom-left, top-right, bottom-right.
    let corner = vec2<f32>(f32(vi >> 1u), f32(vi & 1u));
    var r = inst.rect;
    if (inst.params.z < 0.5) {
        // Room for the anti-aliased edge outside the shape.
        let grow = inst.params.y + 1.0;
        r = r + vec4<f32>(-grow, -grow, grow, grow);
    }
    let px = mix(r.xy, r.zw, corner);
    var out: VertexOutput;
    out.pos = vec4<f32>(px.x / u.frame_size.x * 2.0 - 1.0, 1.0 - px.y / u.frame_size.y * 2.0, 0.0, 1.0);
    out.rect = inst.rect;
    out.radius = inst.radius;
    out.fill_color = inst.fill_color;
    out.stroke_color = inst.stroke_color;
    out.params = inst.params;
    out.tex = mix(inst.uv.xy, inst.uv.zw, corner);
    return out;
}

// distance for one quadrant, as in round_rectangle.wgsl (y up).
fn calc_distance(p: vec2<f32>, b: vec2<f32>, r: vec4<f32>) -> f32 {
    let rr = select(r.zw, r.xy, p.x > 0.0);
    let rx = select(rr.y, rr.x, p.y > 0.0);
    let d = abs(p) - b + rx;
    return min(max(d.x, d.y), 0.0) + length(max(d, vec2<f32>(0.0))) - rx;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    if (in.params.z > 1.5) {
        let p = in.pos.xy;
        let r = in.rect;
        if (p.x < r.x || p.x > r.z || p.y < r.y || p.y > r.w) {
            discard;
        }
        let sw = in.params.x;
        if (p.x >= r.z - sw || p.x <= r.x + sw || p.y <= r.y + sw || p.y >= r.w - sw) {
            return in.stroke_color;
        }
        return in.fill_color;
    }
    if (in.params.z > 0.5) {
        // textureSampleLevel, not textureSample: this branch depends on a
        // per-instance value, and WGSL allows implicit-derivative sampling
        // only in uniform control flow. Chrome's WebGPU (Tint) rejects the
        // whole shader module otherwise, and nothing draws (#333). The atlas has a
        // single mip level, so level 0 is what textureSample would pick.
        let coverage = textureSampleLevel(atlas, samp, in.tex, 0.0).a;
        let a = in.fill_color.a * coverage;
        if (!(a > 0.0)) {
            discard;
        }
        return vec4<f32>(in.fill_color.rgb, a);
    }

    let stroke_half = in.params.x;
    let softness = in.params.y;
    let center = (in.rect.xy + in.rect.zw) * 0.5;
    // y up, as the round_rectangle shader measures.
    let p = vec2<f32>(in.pos.x - center.x, center.y - in.pos.y);
    let size_half = (in.rect.zw - in.rect.xy) * 0.5 - vec2<f32>(stroke_half);
    let distance = calc_distance(p, size_half, in.radius - vec4<f32>(stroke_half));
    let alpha = 1.0 - smoothstep(stroke_half - softness, stroke_half + softness, distance);
    var color = in.fill_color;
    if (stroke_half > 0.0) {
        let blend = smoothstep(-stroke_half - softness, -stroke_half + softness, distance);
        color = mix(in.fill_color, in.stroke_color, blend);
    }
    if (!(alpha > 0.0)) {
        discard;
    }
    return vec4<f32>(color.rgb, color.a * alpha);
}
