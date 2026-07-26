// rectangle — sharp-cornered fill + stroke via SDF. Port of rectangle.vert/rectangle.frag.

struct Uniforms {
    frame_size: vec2<f32>,        // offset 0  (framebuffer pixels)
    stroke_width: f32,            // offset 8
    rect_coords: vec4<f32>,       // offset 16 (x1, x2, y1, y2)
    fill_color: vec4<f32>,        // offset 32
    stroke_color: vec4<f32>,      // offset 48
    inv_transform: mat3x3<f32>,   // offset 64 (16-byte column stride)
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

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    // reconstruct GL-convention gl_FragCoord (y-up) from WebGPU position (y-down)
    let frag_coord = vec2<f32>(in.pos.x, u.frame_size.y - in.pos.y);
    let frag_pos = (u.inv_transform * vec3<f32>(frag_coord, 1.0)).xy;

    // discard if outside rectangle coords, necessary to draw thin stroke and
    // mitigate inconsistent borders issue
    if (frag_pos.x < u.rect_coords[0] || frag_pos.x > u.rect_coords[1] ||
        frag_pos.y < u.frame_size.y - u.rect_coords[3] || frag_pos.y > u.frame_size.y - u.rect_coords[2]) {
        discard;
    }

    var color = u.fill_color;

    if (frag_pos.x >= u.rect_coords[1] - u.stroke_width) {
        color = u.stroke_color;
    } else if (frag_pos.x <= u.rect_coords[0] + u.stroke_width) {
        color = u.stroke_color;
    } else if (frag_pos.y <= u.frame_size.y - u.rect_coords[3] + u.stroke_width) {
        color = u.stroke_color;
    } else if (frag_pos.y >= u.frame_size.y - u.rect_coords[2] - u.stroke_width) {
        color = u.stroke_color;
    }

    return color;
}
