struct gl_Light {
    float4 gl_color;
    float gl_intensity;
    int _end_pad_0;
    int _end_pad_1;
    int _end_pad_2;
};

struct gl_VertexOut {
    float4 gl_pos : SV_Position;
    float2 gl_uv : LOC0;
};

static const float gl_SCALE = 2.0;

cbuffer gl_light : register(b0) { gl_Light gl_light; }
static int gl_counter = (int)0;

struct VertexOutput_vs_main {
    float2 gl_uv : LOC0;
    float4 gl_pos : SV_Position;
};

struct FragmentInput_fs_main {
    float2 gl_uv_1 : LOC0;
    float4 gl_pos_1 : SV_Position;
};

float4 gl_shade(gl_Light gl_l, float _group_0_binding_0_fs)
{
    float4 gl_acc = (float4)0;

    float4 gl_c = ((gl_l.gl_color * gl_l.gl_intensity) * gl_SCALE);
    gl_acc = (gl_c * _group_0_binding_0_fs);
    float4 _e9 = gl_acc;
    return _e9;
}

VertexOutput_vs_main vs_main(float2 gl_in : LOC0)
{
    gl_VertexOut gl_o = (gl_VertexOut)0;

    gl_o.gl_pos = float4(gl_in, 0.0, 1.0);
    gl_o.gl_uv = gl_in;
    gl_VertexOut _e7 = gl_o;
    const gl_VertexOut gl_vertexout = _e7;
    const VertexOutput_vs_main gl_vertexout_1 = { gl_vertexout.gl_uv, gl_vertexout.gl_pos };
    return gl_vertexout_1;
}

float4 fs_main(FragmentInput_fs_main fragmentinput_fs_main) : SV_Target0
{
    gl_VertexOut gl_v = { fragmentinput_fs_main.gl_pos_1, fragmentinput_fs_main.gl_uv_1 };
    int _e2 = gl_counter;
    gl_counter = asint(asuint(_e2) + asuint(int(1)));
    int _e6 = gl_counter;
    float _immediates_binding_fs = float(_e6);
    gl_Light _e9 = gl_light;
    const float4 _e10 = gl_shade(_e9, _immediates_binding_fs);
    return (_e10 + float4(gl_v.gl_uv, 0.0, 0.0));
}
