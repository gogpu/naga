#version 330 core
struct VertexOut {
    vec4 position;
    float value;
    vec4 unused_value2_;
    float unused_value;
    float value2_;
};
smooth out float _vs2fs_location1;
smooth out vec4 _vs2fs_location2;
smooth out float _vs2fs_location0;
smooth out float _vs2fs_location3;

void main() {
    VertexOut _tmp_return = VertexOut(vec4(1.0), 1.0, vec4(2.0), 1.0, 0.5);
    gl_Position = _tmp_return.position;
    _vs2fs_location1 = _tmp_return.value;
    _vs2fs_location2 = _tmp_return.unused_value2_;
    _vs2fs_location0 = _tmp_return.unused_value;
    _vs2fs_location3 = _tmp_return.value2_;
    return;
}

