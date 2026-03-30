// === Entry Point: entry_point_one (fragment) ===
#version 330 core
uniform sampler2D _group_0_binding_0_fs;

layout(location = 0) out vec4 _fs2p_location0;

void main() {
    vec4 pos = gl_FragCoord;
    vec4 _e4 = texture(_group_0_binding_0_fs, vec2(pos.xy));
    _fs2p_location0 = _e4;
    return;
}


// === Entry Point: entry_point_two (fragment) ===
#version 330 core
uniform sampler2D _group_0_binding_0_fs;

layout(std140) uniform type_2_block_0Fragment { vec2 _group_0_binding_4_fs; };

layout(location = 0) out vec4 _fs2p_location0;

void main() {
    vec2 _e3 = _group_0_binding_4_fs;
    vec4 _e4 = texture(_group_0_binding_0_fs, vec2(_e3));
    _fs2p_location0 = _e4;
    return;
}


// === Entry Point: entry_point_three (fragment) ===
#version 330 core
uniform sampler2D _group_0_binding_0_fs;

uniform sampler2D _group_0_binding_1_fs;

layout(std140) uniform type_2_block_0Fragment { vec2 _group_0_binding_4_fs; };

layout(std140) uniform type_2_block_1Fragment { vec2 _group_1_binding_0_fs; };

layout(location = 0) out vec4 _fs2p_location0;

void main() {
    vec2 _e3 = _group_1_binding_0_fs;
    vec2 _e5 = _group_0_binding_4_fs;
    vec4 _e7 = texture(_group_0_binding_0_fs, vec2((_e3 + _e5)));
    vec2 _e11 = _group_0_binding_4_fs;
    vec4 _e12 = texture(_group_0_binding_1_fs, vec2(_e11));
    _fs2p_location0 = (_e7 + _e12);
    return;
}

