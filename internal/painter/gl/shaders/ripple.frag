#version 110

uniform vec2 frame_size;
uniform vec4 rect_coords;       // x1, x2, y1, y2
uniform vec2 rect_size_half;    // half width/height of rect
uniform float radius;           // per-corner radius (uniform)
uniform float edge_softness;

uniform vec2 ripple_center;     // center in screen pixels
uniform float ripple_radius;    // current radius in screen pixels
uniform vec4 ripple_color;      // RGBA with target opacity
uniform mat3 inv_transform;

void main() {
    vec2 frag_pos = (inv_transform * vec3(gl_FragCoord.xy, 1.0)).xy;

    // Transform to centered rect coordinates (same as round_rectangle)
    vec4 frag_rect_coords = vec4(
        rect_coords[0], rect_coords[1],
        frame_size.y - rect_coords[3],
        frame_size.y - rect_coords[2]
    );
    vec2 rect_center = vec2(
        (frag_rect_coords[0] + frag_rect_coords[1]) * 0.5,
        (frag_rect_coords[2] + frag_rect_coords[3]) * 0.5
    );
    vec2 p = frag_pos - rect_center;

    // Rounded rectangle SDF for clipping
    float r = radius;
    vec2 d = abs(p) - rect_size_half + r;
    float rect_dist = min(max(d.x, d.y), 0.0) + length(max(d, 0.0)) - r;
    float clip_mask = 1.0 - smoothstep(-edge_softness, edge_softness, rect_dist);

    // Radial ripple: solid circle from center
    float dist_to_center = length(frag_pos - ripple_center);
    float ripple_mask = 1.0 - smoothstep(ripple_radius - edge_softness, ripple_radius + edge_softness, dist_to_center);

    // Combine: ripple clipped to rounded rectangle
    float alpha = ripple_color.a * ripple_mask * clip_mask;
    if (alpha < 0.005)
        discard;

    gl_FragColor = vec4(ripple_color.rgb * alpha, alpha);
}
