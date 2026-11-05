#version 100

#ifdef GL_ES
# ifdef GL_FRAGMENT_PRECISION_HIGH
precision highp float;
# else
precision mediump float;
#endif
precision mediump int;
precision lowp sampler2D;
#endif

// gradient: canvas.LinearGradient and canvas.RadialGradient computed per
// fragment (#957), in place of a CPU-generated texture. A port of the WebGPU
// painter's shaders/gradient.wgsl (#956), so both reproduce
// canvas.(*LinearGradient).Generate and canvas.(*RadialGradient).Generate:
// the same distance d(x, y) over the object's pixels (sampled at pixel
// centres, top-left origin), and the same colour, premultiplied RGBA
// interpolated between the stops that bracket d, clamped at both ends.
//
// gradient_es.frag is the same shader with the GLSL ES header. Its uniforms
// take 15 vectors, under OpenGL ES 2's guaranteed 16 per fragment shader.

uniform vec4 geom;           // object width, height, left x, top y (framebuffer pixels, y up)
uniform vec4 params;         // kind (0 linear, 1 radial), stop count (2..6), alpha, unused
uniform vec4 linear;         // kx, ky, k0, den: d = |kx*x + ky*y + k0| / den
uniform vec4 radial;         // cx, cy, a, b: ellipse centre and semi-axes
uniform mat3 inv_transform;
uniform vec4 stop_pos[2];    // six positions, ascending, in [0, 1]
uniform vec4 stop_col[6];    // premultiplied RGBA

// stopPosition is stop i's position. GLSL ES 1.00 indexes a vector by a
// constant only, so the component is picked by a mask.
float stopPosition(int i) {
    vec4 pick = vec4(equal(vec4(0.0, 1.0, 2.0, 3.0), vec4(float(i < 4 ? i : i - 4))));
    return i < 4 ? dot(stop_pos[0], pick) : dot(stop_pos[1], pick);
}

void main() {
    vec2 frag_pos = (inv_transform * vec3(gl_FragCoord.xy, 1.0)).xy;
    // Object-local pixel coordinates, top-left origin, as Generate's x and y.
    float x = frag_pos.x - geom.z;
    float y = geom.w - frag_pos.y;
    // The quad carries the shapes' anti-aliasing margin; a gradient is a
    // plain image, so nothing outside its own pixels is drawn.
    if (x < 0.0 || y < 0.0 || x >= geom.x || y >= geom.y) {
        discard;
    }

    float d;
    if (params.x < 0.5) {
        d = abs(linear.x * x + linear.y * y + linear.z) / linear.w;
    } else {
        float dx = radial.x - x;
        float dy = radial.y - y;
        float a = radial.z;
        float b = radial.w;
        d = min(sqrt(dx * dx + dy * dy * a * a / (b * b)) / a, 1.0);
    }

    int n = int(params.y + 0.5);
    vec4 col = stop_col[0];
    if (d > stopPosition(0)) {
        for (int i = 1; i < 6; i++) {
            if (i >= n) {
                break;
            }
            float p1 = stopPosition(i);
            if (d <= p1) {
                float p0 = stopPosition(i - 1);
                float seg = p1 - p0;
                col = seg <= 0.0 ? stop_col[i] : mix(stop_col[i - 1], stop_col[i], (d - p0) / seg);
                break;
            }
            col = stop_col[i];
        }
    }
    gl_FragColor = col * params.z;
}
