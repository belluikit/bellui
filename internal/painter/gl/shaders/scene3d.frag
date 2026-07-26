#version 110

varying vec3 vNormal;
varying vec4 vColor;

uniform vec3 light_dir;
uniform vec3 light_color;
uniform float ambient;

void main() {
    vec3 n = normalize(vNormal);
    float diffuse = max(dot(n, -light_dir), 0.0);
    vec3 lighting = light_color * (ambient + (1.0 - ambient) * diffuse);
    gl_FragColor = vec4(vColor.rgb * lighting, vColor.a);
}
