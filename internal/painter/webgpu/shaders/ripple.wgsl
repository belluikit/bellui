// ripple — MD3 radial touch ripple clipped to a rounded rect. Port of rectangle.vert/ripple.frag.

struct Uniforms {
    frame_size: vec2<f32>,        // offset 0  (framebuffer pixels)
    edge_softness: f32,           // offset 8
    ripple_radius: f32,           // offset 12 (current radius in screen pixels)
    rect_coords: vec4<f32>,       // offset 16 (x1, x2, y1, y2)
    rect_size_half: vec2<f32>,    // offset 32 (half width/height of rect)
    ripple_center: vec2<f32>,     // offset 40 (center in screen pixels)
    radius: vec4<f32>,            // offset 48 (corner radii: tr, br, tl, bl)
    ripple_color: vec4<f32>,      // offset 64 (RGBA with target opacity)
    inv_transform: mat3x3<f32>,   // offset 80 (16-byte column stride)
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

// distance is calculated for a single quadrant, as in round_rectangle
// returns invalid output if corner radius exceeds half of the shorter edge
fn calc_distance(p: vec2<f32>, b: vec2<f32>, r: vec4<f32>) -> f32 {
    let rr = select(r.zw, r.xy, p.x > 0.0);
    let rx = select(rr.y, rr.x, p.y > 0.0);

    let d = abs(p) - b + rx;
    return min(max(d.x, d.y), 0.0) + length(max(d, vec2<f32>(0.0))) - rx;
}

// distance is calculated for all necessary quadrants, as in round_rectangle
// corner radius may exceed half of the shorter edge
fn calc_distance_all_quadrants(p: vec2<f32>, size: vec2<f32>, radius: vec4<f32>) -> f32 {
    let d = abs(p) - size;
    var dist = length(max(d, vec2<f32>(0.0))) + min(max(d.x, d.y), 0.0);

    let p_tl = p - vec2<f32>(radius.z - size.x, size.y - radius.z);
    if (p_tl.x < 0.0 && p_tl.y > 0.0) { dist = max(dist, length(p_tl) - radius.z); }

    let p_tr = p - vec2<f32>(size.x - radius.x, size.y - radius.x);
    if (p_tr.x > 0.0 && p_tr.y > 0.0) { dist = max(dist, length(p_tr) - radius.x); }

    let p_br = p - vec2<f32>(size.x - radius.y, radius.y - size.y);
    if (p_br.x > 0.0 && p_br.y < 0.0) { dist = max(dist, length(p_br) - radius.y); }

    let p_bl = p - vec2<f32>(radius.w - size.x, radius.w - size.y);
    if (p_bl.x < 0.0 && p_bl.y < 0.0) { dist = max(dist, length(p_bl) - radius.w); }

    return dist;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    let frag_coord = vec2<f32>(in.pos.x, u.frame_size.y - in.pos.y);
    let frag_pos = (u.inv_transform * vec3<f32>(frag_coord, 1.0)).xy;

    // transform to centered rect coordinates (same as round_rectangle)
    let frag_rect_coords = vec4<f32>(
        u.rect_coords[0], u.rect_coords[1],
        u.frame_size.y - u.rect_coords[3],
        u.frame_size.y - u.rect_coords[2]
    );
    let rect_center = vec2<f32>(
        (frag_rect_coords[0] + frag_rect_coords[1]) * 0.5,
        (frag_rect_coords[2] + frag_rect_coords[3]) * 0.5
    );
    let p = frag_pos - rect_center;

    // rounded rectangle SDF for clipping, each corner its own radius
    let max_radius = max(max(u.radius.x, u.radius.y), max(u.radius.z, u.radius.w));
    var rect_dist: f32;
    if (max_radius - 0.9 > min(u.rect_size_half.x, u.rect_size_half.y)) {
        rect_dist = calc_distance_all_quadrants(p, u.rect_size_half, u.radius);
    } else {
        rect_dist = calc_distance(p, u.rect_size_half, u.radius);
    }
    let clip_mask = 1.0 - smoothstep(-u.edge_softness, u.edge_softness, rect_dist);

    // radial ripple: solid circle from center
    let dist_to_center = length(frag_pos - u.ripple_center);
    let ripple_mask = 1.0 - smoothstep(u.ripple_radius - u.edge_softness, u.ripple_radius + u.edge_softness, dist_to_center);

    // combine: ripple clipped to rounded rectangle
    let alpha = u.ripple_color.a * ripple_mask * clip_mask;
    if (alpha < 0.005) {
        discard;
    }

    return vec4<f32>(u.ripple_color.rgb * alpha, alpha);
}
