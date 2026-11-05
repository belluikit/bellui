// gradient — canvas.LinearGradient and canvas.RadialGradient computed per
// fragment (#956), in place of a CPU-generated texture. It reproduces
// canvas.(*LinearGradient).Generate and canvas.(*RadialGradient).Generate:
// the same distance function d(x, y) over the object's pixels (sampled at
// pixel centres, top-left origin), and the same colour: premultiplied RGBA
// interpolated between the stops that bracket d, stops clamped at both ends.

struct Uniforms {
    frame_size: vec2<f32>,        // offset 0   (framebuffer pixels)
    size: vec2<f32>,              // offset 8   (object size, framebuffer pixels)
    origin: vec2<f32>,            // offset 16  (object top-left, bottom-up framebuffer pixels)
    kind: f32,                    // offset 24  (0 linear, 1 radial)
    stop_count: f32,              // offset 28  (2..6)
    linear: vec4<f32>,            // offset 32  (kx, ky, k0, den): d = |kx*x + ky*y + k0| / den
    radial: vec4<f32>,            // offset 48  (cx, cy, a, b): ellipse centre and semi-axes
    alpha: f32,                   // offset 64  (group opacity)
    inv_transform: mat3x3<f32>,   // offset 80  (16-byte column stride)
    stop_pos: array<vec4<f32>, 2>, // offset 128 (6 positions used, ascending, in [0, 1])
    stop_col: array<vec4<f32>, 6>, // offset 160 (premultiplied RGBA); 256 bytes in all, the arena's block stride
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

fn stop_position(i: i32) -> f32 {
    return u.stop_pos[i / 4][i % 4];
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    let frag_coord = vec2<f32>(in.pos.x, u.frame_size.y - in.pos.y);
    let frag_pos = (u.inv_transform * vec3<f32>(frag_coord, 1.0)).xy;
    // Object-local pixel coordinates, top-left origin, as Generate's x and y.
    let x = frag_pos.x - u.origin.x;
    let y = u.origin.y - frag_pos.y;
    // The quad carries the shapes' anti-aliasing margin; a gradient is a
    // plain image, so nothing outside its own pixels is drawn.
    if (x < 0.0 || y < 0.0 || x >= u.size.x || y >= u.size.y) {
        discard;
    }

    var d: f32;
    if (u.kind < 0.5) {
        d = abs(u.linear.x * x + u.linear.y * y + u.linear.z) / u.linear.w;
    } else {
        let dx = u.radial.x - x;
        let dy = u.radial.y - y;
        let a = u.radial.z;
        let b = u.radial.w;
        let da = sqrt(dx * dx + dy * dy * a * a / (b * b));
        d = min(da / a, 1.0);
    }

    let n = i32(u.stop_count);
    var col: vec4<f32>;
    if (d <= stop_position(0)) {
        col = u.stop_col[0];
    } else if (d >= stop_position(n - 1)) {
        col = u.stop_col[n - 1];
    } else {
        col = u.stop_col[n - 1];
        for (var i: i32 = 1; i < n; i = i + 1) {
            let p1 = stop_position(i);
            if (d <= p1) {
                let p0 = stop_position(i - 1);
                let seg = p1 - p0;
                if (seg <= 0.0) {
                    col = u.stop_col[i];
                } else {
                    col = mix(u.stop_col[i - 1], u.stop_col[i], (d - p0) / seg);
                }
                break;
            }
        }
    }
    return col * u.alpha;
}
