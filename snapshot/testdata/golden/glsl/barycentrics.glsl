// === Entry Point: fs_main (fragment) ===
#version 330 core
#extension GL_EXT_fragment_shader_barycentric : require
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    vec3 bary = gl_BaryCoordEXT;
    _fs2p_location0 = vec4(bary, 1.0);
    return;
}


// === Entry Point: fs_main_no_perspective (fragment) ===
#version 330 core
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    vec3 bary_1 = gl_FragCoord;
    _fs2p_location0 = vec4(bary_1, 1.0);
    return;
}

