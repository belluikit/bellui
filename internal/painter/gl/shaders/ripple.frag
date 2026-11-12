#version 110

uniform vec2 frame_size;
uniform vec4 rect_coords;       // x1, x2, y1, y2
uniform vec2 rect_size_half;    // half width/height of rect
uniform vec4 radius;            // corner radii: tr, br, tl, bl
uniform float edge_softness;

uniform vec2 ripple_center;     // center in screen pixels
uniform float ripple_radius;    // current radius in screen pixels
uniform vec4 ripple_color;      // RGBA with target opacity
uniform mat3 inv_transform;

// distance is calculated for a single quadrant, as in round_rectangle
// returns invalid output if corner radius exceeds half of the shorter edge
float calc_distance(vec2 p, vec2 b, vec4 r)
{
    r.xy = (p.x > 0.0) ? r.xy : r.zw;
    r.x = (p.y > 0.0) ? r.x : r.y;

    vec2 d = abs(p) - b + r.x;
    return min(max(d.x, d.y), 0.0) + length(max(d, 0.0)) - r.x;
}

// distance is calculated for all necessary quadrants, as in round_rectangle
// corner radius may exceed half of the shorter edge
float calc_distance_all_quadrants(vec2 p, vec2 size, vec4 radius)
{
    vec2 d = abs(p) - size;
    float dist = length(max(d, 0.0)) + min(max(d.x, d.y), 0.0);

    vec2 p_tl = p - vec2(radius.z - size.x, size.y - radius.z);
    if (p_tl.x < 0.0 && p_tl.y > 0.0) dist = max(dist, length(p_tl) - radius.z);

    vec2 p_tr = p - vec2(size.x - radius.x, size.y - radius.x);
    if (p_tr.x > 0.0 && p_tr.y > 0.0) dist = max(dist, length(p_tr) - radius.x);

    vec2 p_br = p - vec2(size.x - radius.y, radius.y - size.y);
    if (p_br.x > 0.0 && p_br.y < 0.0) dist = max(dist, length(p_br) - radius.y);

    vec2 p_bl = p - vec2(radius.w - size.x, radius.w - size.y);
    if (p_bl.x < 0.0 && p_bl.y < 0.0) dist = max(dist, length(p_bl) - radius.w);

    return dist;
}

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

    // Rounded rectangle SDF for clipping, each corner its own radius
    float max_radius = max(max(radius.x, radius.y), max(radius.z, radius.w));
    float rect_dist;
    if (max_radius - 0.9 > min(rect_size_half.x, rect_size_half.y))
        rect_dist = calc_distance_all_quadrants(p, rect_size_half, radius);
    else
        rect_dist = calc_distance(p, rect_size_half, radius);
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
