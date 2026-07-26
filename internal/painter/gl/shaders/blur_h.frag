#version 110

uniform sampler2D tex;
uniform vec2 resolution;
uniform float blur_radius;

varying vec2 fragTexCoord;

void main() {
    float step_x = 1.0 / resolution.x;
    vec4 sum = vec4(0.0);

    // 9-tap Gaussian kernel weights (sigma ~= blur_radius/3)
    sum += texture2D(tex, fragTexCoord + vec2(-4.0 * step_x * blur_radius, 0.0)) * 0.0162;
    sum += texture2D(tex, fragTexCoord + vec2(-3.0 * step_x * blur_radius, 0.0)) * 0.0540;
    sum += texture2D(tex, fragTexCoord + vec2(-2.0 * step_x * blur_radius, 0.0)) * 0.1216;
    sum += texture2D(tex, fragTexCoord + vec2(-1.0 * step_x * blur_radius, 0.0)) * 0.1945;
    sum += texture2D(tex, fragTexCoord) * 0.2270;
    sum += texture2D(tex, fragTexCoord + vec2( 1.0 * step_x * blur_radius, 0.0)) * 0.1945;
    sum += texture2D(tex, fragTexCoord + vec2( 2.0 * step_x * blur_radius, 0.0)) * 0.1216;
    sum += texture2D(tex, fragTexCoord + vec2( 3.0 * step_x * blur_radius, 0.0)) * 0.0540;
    sum += texture2D(tex, fragTexCoord + vec2( 4.0 * step_x * blur_radius, 0.0)) * 0.0162;

    gl_FragColor = sum;
}
