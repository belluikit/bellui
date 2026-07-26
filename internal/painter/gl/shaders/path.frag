#version 110

uniform vec4 fill_color;

varying float v_alpha;

void main() {
    gl_FragColor = vec4(fill_color.rgb, fill_color.a * v_alpha);
}
