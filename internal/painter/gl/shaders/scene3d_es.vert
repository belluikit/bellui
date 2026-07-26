#version 100

#ifdef GL_ES
# ifdef GL_FRAGMENT_PRECISION_HIGH
precision highp float;
# else
precision mediump float;
#endif
precision mediump int;
#endif

attribute vec3 vert;
attribute vec3 normal;
attribute vec4 color;

uniform mat4 mvp;

varying vec3 vNormal;
varying vec4 vColor;

void main() {
    gl_Position = mvp * vec4(vert, 1.0);
    vNormal = normal;
    vColor = color;
}
