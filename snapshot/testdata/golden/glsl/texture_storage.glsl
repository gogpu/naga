#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 8, local_size_y = 8, local_size_z = 1) in;

layout(rgba8) writeonly uniform image2D _group_0_binding_0_cs;


void main() {
    uvec3 gid = gl_GlobalInvocationID;
    bool unnamed = false;
    uvec2 dims = uvec2(imageSize(_group_0_binding_0_cs).xy);
    if (!((gid.x >= dims.x))) {
        unnamed = (gid.y >= dims.y);
    } else {
        unnamed = true;
    }
    bool _e13 = unnamed;
    if (_e13) {
        return;
    }
    vec2 uv = vec2((float(gid.x) / float(dims.x)), (float(gid.y) / float(dims.y)));
    vec4 color = vec4(uv.x, uv.y, 0.5, 1.0);
    imageStore(_group_0_binding_0_cs, ivec2(gid.xy), color);
    return;
}

