#version 110

attribute vec2 vert;
attribute float alpha;

varying float v_alpha;

void main() {
    gl_Position = vec4(vert, 0, 1);
    v_alpha = alpha;
}
