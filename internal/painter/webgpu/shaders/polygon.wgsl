// polygon — regular N-gon fill + stroke via SDF. Port of rectangle.vert/polygon.frag.

struct Uniforms {
    frame_size: vec2<f32>,        // offset 0  (framebuffer pixels)
    edge_softness: f32,           // offset 8
    outer_radius: f32,            // offset 12
    rect_coords: vec4<f32>,       // offset 16
    angle: f32,                   // offset 32 (degrees)
    sides: f32,                   // offset 36
    corner_radius: f32,           // offset 40
    stroke_width: f32,            // offset 44
    fill_color: vec4<f32>,        // offset 48
    stroke_color: vec4<f32>,      // offset 64
    inv_transform: mat3x3<f32>,   // offset 80 (16-byte column stride)
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

// GLSL-compatible modulo: x - y * floor(x / y) (differs from WGSL's % for negatives).
fn glmod(x: f32, y: f32) -> f32 {
    return x - y * floor(x / y);
}

fn rotate(angle: f32) -> mat2x2<f32> {
    let s = sin(-angle);
    let c = cos(-angle);
    return mat2x2<f32>(c, -s, s, c);
}

// The signed distance from the point to the regular polygon's edge
fn regular_distance(p: vec2<f32>, r: f32, s: i32) -> f32 {
    let angle = PI / f32(s);
    let angle_cos = cos(angle);
    let angle_sin = sin(angle);
    let angular_offset = glmod(atan2(p.x, p.y), 2.0 * angle) - angle;
    var distance = length(p) * vec2<f32>(cos(angular_offset), abs(sin(angular_offset))) - r * vec2<f32>(angle_cos, angle_sin);
    distance.y = distance.y + clamp(-distance.y, 0.0, r * angle_sin);
    return length(distance) * sign(distance.x);
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    let frag_coord = vec2<f32>(in.pos.x, u.frame_size.y - in.pos.y);
    let frag_pos = (u.inv_transform * vec3<f32>(frag_coord, 1.0)).xy;
    let frag_rect_coords = vec4<f32>(u.rect_coords[0], u.rect_coords[1], u.frame_size.y - u.rect_coords[3], u.frame_size.y - u.rect_coords[2]);
    var vec_centered_pos = frag_pos - vec2<f32>(frag_rect_coords[0] + frag_rect_coords[1], frag_rect_coords[2] + frag_rect_coords[3]) * 0.5;

    vec_centered_pos = rotate(radians(u.angle)) * vec_centered_pos;
    let dist = regular_distance(vec_centered_pos, u.outer_radius - u.corner_radius, i32(u.sides)) - u.corner_radius;
    var final_color = u.fill_color;

    if (u.stroke_width > 0.0) {
        // mask for the fill area (inside, shrunk by stroke width)
        let fill_mask = smoothstep(-u.stroke_width + u.edge_softness, -u.stroke_width - u.edge_softness, dist);
        final_color = mix(u.stroke_color, u.fill_color, fill_mask);
    }

    // smooth edges
    let final_alpha = smoothstep(u.edge_softness, -u.edge_softness, dist);
    return vec4<f32>(final_color.rgb, final_color.a * final_alpha);
}
