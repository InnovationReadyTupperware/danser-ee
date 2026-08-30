#version 330
precision highp float;

uniform sampler2DArray tex;
uniform mat4x3 rgbToYuv;

in vec2 tex_coord;

layout(location = 0) out float outY;
layout(location = 1) out float outU;
layout(location = 2) out float outV;

void main() {
    vec3 src = texture(tex, vec3(tex_coord.x, 1 - tex_coord.y, 0)).rgb;
    vec3 color = rgbToYuv*vec4(src, 1);
    outY = color.r;
    outU = color.g;
    outV = color.b;
}
