// arc — rounded annular arc / ring fill + stroke via SDF. Port of rectangle.vert/arc.frag.
// Operates in the unit-circle coordinate system; angles measured from +X axis.

struct Uniforms {
    frame_size: vec2<f32>,        // offset 0  (framebuffer pixels)
    edge_softness: f32,           // offset 8
    inner_radius: f32,            // offset 12
    rect_coords: vec4<f32>,       // offset 16
    outer_radius: f32,            // offset 32
    start_angle: f32,             // offset 36 (degrees)
    end_angle: f32,               // offset 40 (degrees)
    corner_radius: f32,           // offset 44
    fill_color: vec4<f32>,        // offset 48
    stroke_width: f32,            // offset 64
    stroke_color: vec4<f32>,      // offset 80
    inv_transform: mat3x3<f32>,   // offset 96 (16-byte column stride)
};

@group(0) @binding(0) var<uniform> u: Uniforms;

const PI: f32 = 3.141592653589793;

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

// Signed distance for a rounded arc shape.
//   p  - coordinate to evaluate     r1/r2 - inner/outer radius
//   a0/a1 - start/end angle (rad)   cr    - corner rounding radius
// Negative inside, positive outside, zero on the edge.
fn sd_rounded_arc(p_in: vec2<f32>, r1: f32, r2: f32, a0: f32, a1: f32, cr: f32) -> f32 {
    // center the arc for simpler calculations
    let mid_angle = (a0 + a1) / 2.0;
    let arc_span = abs(a1 - a0);

    let cs = cos(mid_angle);
    let sn = sin(mid_angle);
    let p = mat2x2<f32>(cs, -sn, sn, cs) * p_in;

    // distance to a rounded box in a pseudo-polar space
    let r = length(p);
    // atan2(y, x) for standard angle convention (0 degrees = right)
    let a = atan2(p.y, p.x);

    let box_half_size = vec2<f32>(arc_span * 0.5 * r, (r2 - r1) * 0.5);
    let q = vec2<f32>(a * r, r - (r1 + r2) * 0.5);

    // inner corner radius clamped to half the smaller dimension to prevent
    // overlapping corners; outer corner radius is just cr
    let inner_cr = min(cr, 0.5 * min(r2 - r1, arc_span * r1));
    let outer_cr = cr;

    // interpolate inner→outer corner radius by radial position (0 inner, 1 outer)
    let t = smoothstep(-box_half_size.y, box_half_size.y, q.y);
    let effective_cr = mix(inner_cr, outer_cr, t);

    let dist = abs(q) - box_half_size + effective_cr;
    return length(max(dist, vec2<f32>(0.0))) + min(max(dist.x, dist.y), 0.0) - effective_cr;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    let frag_coord = vec2<f32>(in.pos.x, u.frame_size.y - in.pos.y);
    let frag_pos = (u.inv_transform * vec3<f32>(frag_coord, 1.0)).xy;
    let frag_rect_coords = vec4<f32>(u.rect_coords[0], u.rect_coords[1], u.frame_size.y - u.rect_coords[3], u.frame_size.y - u.rect_coords[2]);
    let vec_centered_pos = frag_pos - vec2<f32>(frag_rect_coords[0] + frag_rect_coords[1], frag_rect_coords[2] + frag_rect_coords[3]) * 0.5;
    let start_rad = radians(u.start_angle);
    let end_rad = radians(u.end_angle);

    // the sd_rounded_arc function creates a segment at the start/end angle,
    // undesirable for a complete circle, so handle full circles separately
    var dist: f32;
    if (abs(end_rad - start_rad) >= 2.0 * PI - 0.001) {
        let r = length(vec_centered_pos);
        if (u.inner_radius < 0.5) {
            // no inner radius
            dist = r - u.outer_radius;
        } else {
            let ring_center_radius = (u.inner_radius + u.outer_radius) * 0.5;
            let ring_thickness = (u.outer_radius - u.inner_radius) * 0.5;
            dist = abs(r - ring_center_radius) - ring_thickness;
        }
    } else {
        dist = sd_rounded_arc(vec_centered_pos, u.inner_radius, u.outer_radius, start_rad, end_rad, u.corner_radius);
    }

    var final_color = u.fill_color;

    if (u.stroke_width > 0.0) {
        let fill_mask = smoothstep(u.edge_softness, -u.edge_softness, dist + u.stroke_width);
        final_color = mix(u.stroke_color, u.fill_color, fill_mask);
    }

    let final_alpha = smoothstep(u.edge_softness, -u.edge_softness, dist);
    return vec4<f32>(final_color.rgb, final_color.a * final_alpha);
}
