#version 110

uniform sampler2D tex;
uniform vec2 resolution;
uniform float blur_radius; // the distance between taps, in texels

varying vec2 fragTexCoord;

void main() {
    float step_x = 1.0 / resolution.x;
    vec4 sum = vec4(0.0);

    // 9-tap Gaussian kernel weights. They have a standard deviation of
    // 1.689 taps, so sigma = 1.689 * blur_radius texels; the painters choose
    // blur_radius with paint.PlanBlur.
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
