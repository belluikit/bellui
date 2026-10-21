#version 110

uniform sampler2D tex;
uniform vec2 resolution;
uniform float blur_radius; // the distance between taps, in texels

varying vec2 fragTexCoord;

void main() {
    float step_y = 1.0 / resolution.y;
    vec4 sum = vec4(0.0);

    // 9-tap Gaussian kernel weights (same as horizontal pass)
    sum += texture2D(tex, fragTexCoord + vec2(0.0, -4.0 * step_y * blur_radius)) * 0.0162;
    sum += texture2D(tex, fragTexCoord + vec2(0.0, -3.0 * step_y * blur_radius)) * 0.0540;
    sum += texture2D(tex, fragTexCoord + vec2(0.0, -2.0 * step_y * blur_radius)) * 0.1216;
    sum += texture2D(tex, fragTexCoord + vec2(0.0, -1.0 * step_y * blur_radius)) * 0.1945;
    sum += texture2D(tex, fragTexCoord) * 0.2270;
    sum += texture2D(tex, fragTexCoord + vec2(0.0,  1.0 * step_y * blur_radius)) * 0.1945;
    sum += texture2D(tex, fragTexCoord + vec2(0.0,  2.0 * step_y * blur_radius)) * 0.1216;
    sum += texture2D(tex, fragTexCoord + vec2(0.0,  3.0 * step_y * blur_radius)) * 0.0540;
    sum += texture2D(tex, fragTexCoord + vec2(0.0,  4.0 * step_y * blur_radius)) * 0.0162;

    gl_FragColor = sum;
}
