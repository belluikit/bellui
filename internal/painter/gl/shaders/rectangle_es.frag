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

/* scaled params */
uniform vec2 frame_size;
uniform vec4 rect_coords; //x1 [0], x2 [1], y1 [2], y2 [3]; coords of the rect_frame
uniform float stroke_width;
/* colors params*/
uniform vec4 fill_color;
uniform vec4 stroke_color;
uniform mat3 inv_transform;


void main() {
    vec2 frag_pos = (inv_transform * vec3(gl_FragCoord.xy, 1.0)).xy;

    // discard if outside rectangle coords, necessary to draw thin stroke and mitigate inconsistent borders issue
    if (frag_pos.x < rect_coords[0] || frag_pos.x > rect_coords[1] || frag_pos.y < frame_size.y - rect_coords[3] || frag_pos.y > frame_size.y - rect_coords[2]) {
        discard;
    }

    vec4 color = fill_color;

    if (frag_pos.x >= rect_coords[1] - stroke_width ){
        color = stroke_color;
    } else if (frag_pos.x <= rect_coords[0] + stroke_width){
        color = stroke_color;
    } else if (frag_pos.y <= frame_size.y - rect_coords[3] + stroke_width ){
        color = stroke_color;
    } else if (frag_pos.y >= frame_size.y - rect_coords[2] - stroke_width ){
        color = stroke_color;
    }

    gl_FragColor = color;
}
