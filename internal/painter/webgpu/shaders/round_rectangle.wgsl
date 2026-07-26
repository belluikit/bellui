// round_rectangle — rounded fill + stroke via SDF. Port of rectangle.vert/round_rectangle.frag.

struct Uniforms {
    frame_size: vec2<f32>,        // offset 0  (framebuffer pixels)
    stroke_width_half: f32,       // offset 8
    edge_softness: f32,           // offset 12
    rect_coords: vec4<f32>,       // offset 16 (x1, x2, y1, y2)
    rect_size_half: vec2<f32>,    // offset 32
    radius: vec4<f32>,            // offset 48 (tr, br, tl, bl)
    fill_color: vec4<f32>,        // offset 64
    stroke_color: vec4<f32>,      // offset 80
    inv_transform: mat3x3<f32>,   // offset 96 (16-byte column stride)
};

@group(0) @binding(0) var<uniform> u: Uniforms;

struct VertexInput {
    @location(0) vert: vec2<f32>,
    @location(1) normal: vec2<f32>,
};

struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
};

@vertex
fn vs_main(in: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    out.pos = vec4<f32>(in.vert + in.normal, 0.0, 1.0);
    return out;
}

// distance is calculated for a single quadrant
// returns invalid output if corner radius exceeds half of the shorter edge
fn calc_distance(p: vec2<f32>, b: vec2<f32>, r: vec4<f32>) -> f32 {
    let rr = select(r.zw, r.xy, p.x > 0.0);
    let rx = select(rr.y, rr.x, p.y > 0.0);

    let d = abs(p) - b + rx;
    return min(max(d.x, d.y), 0.0) + length(max(d, vec2<f32>(0.0))) - rx;
}

// distance is calculated for all necessary quadrants
// corner radius may exceed half of the shorter edge
fn calc_distance_all_quadrants(p: vec2<f32>, size: vec2<f32>, radius: vec4<f32>) -> f32 {
    let d = abs(p) - size;
    var dist = length(max(d, vec2<f32>(0.0))) + min(max(d.x, d.y), 0.0);

    // top-left corner
    let p_tl = p - vec2<f32>(radius.z - size.x, size.y - radius.z);
    if (p_tl.x < 0.0 && p_tl.y > 0.0) { dist = max(dist, length(p_tl) - radius.z); }

    // top-right corner
    let p_tr = p - vec2<f32>(size.x - radius.x, size.y - radius.x);
    if (p_tr.x > 0.0 && p_tr.y > 0.0) { dist = max(dist, length(p_tr) - radius.x); }

    // bottom-right corner
    let p_br = p - vec2<f32>(size.x - radius.y, radius.y - size.y);
    if (p_br.x > 0.0 && p_br.y < 0.0) { dist = max(dist, length(p_br) - radius.y); }

    // bottom-left corner
    let p_bl = p - vec2<f32>(radius.w - size.x, radius.w - size.y);
    if (p_bl.x < 0.0 && p_bl.y < 0.0) { dist = max(dist, length(p_bl) - radius.w); }

    return dist;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    let frag_coord = vec2<f32>(in.pos.x, u.frame_size.y - in.pos.y);
    let frag_pos = (u.inv_transform * vec3<f32>(frag_coord, 1.0)).xy;
    let frag_rect_coords = vec4<f32>(u.rect_coords[0], u.rect_coords[1], u.frame_size.y - u.rect_coords[3], u.frame_size.y - u.rect_coords[2]);
    let vec_centered_pos = frag_pos - vec2<f32>(frag_rect_coords[0] + frag_rect_coords[1], frag_rect_coords[2] + frag_rect_coords[3]) * 0.5;

    var distance: f32;
    let max_radius = max(max(u.radius.x, u.radius.y), max(u.radius.z, u.radius.w));
    var final_color = u.fill_color;
    var final_alpha: f32;

    // subtract a small threshold to avoid calling calc_distance_all_quadrants when the
    // largest corner radius is very close to half the rectangle's shortest edge
    if (max_radius - 0.9 > min(u.rect_size_half.x, u.rect_size_half.y) + u.stroke_width_half) {
        // at least one corner radius is larger than half of the shorter edge
        distance = calc_distance_all_quadrants(vec_centered_pos, u.rect_size_half + u.stroke_width_half, u.radius);
        final_alpha = 1.0 - smoothstep(-u.edge_softness, u.edge_softness, distance);

        if (u.stroke_width_half > 0.0) {
            let color_blend = 1.0 - smoothstep(u.stroke_width_half * 2.0 - u.edge_softness, u.stroke_width_half * 2.0 + u.edge_softness, abs(distance));
            final_color = mix(u.fill_color, u.stroke_color, color_blend);
        }
    } else {
        distance = calc_distance(vec_centered_pos, u.rect_size_half, u.radius - u.stroke_width_half);
        final_alpha = 1.0 - smoothstep(u.stroke_width_half - u.edge_softness, u.stroke_width_half + u.edge_softness, distance);

        if (u.stroke_width_half > 0.0) {
            let color_blend = smoothstep(-u.stroke_width_half - u.edge_softness, -u.stroke_width_half + u.edge_softness, distance);
            final_color = mix(u.fill_color, u.stroke_color, color_blend);
        }
    }

    // Discard pixels with non-positive (or NaN) coverage. Zero-alpha pixels
    // contribute nothing under blending; NaN coverage (degenerate distance math
    // on some GPU states) must never reach the framebuffer.
    if (!(final_alpha > 0.0)) {
        discard;
    }

    return vec4<f32>(final_color.rgb, final_color.a * final_alpha);
}
