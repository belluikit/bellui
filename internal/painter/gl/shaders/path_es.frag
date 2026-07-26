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

uniform vec4 fill_color;

varying float v_alpha;

void main() {
    gl_FragColor = vec4(fill_color.rgb, fill_color.a * v_alpha);
}
