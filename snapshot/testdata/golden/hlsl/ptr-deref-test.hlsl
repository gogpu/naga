struct Baz {
    float2 m_0; float2 m_1; float2 m_2;
};

cbuffer baz : register(b1) { Baz baz; }

void pattern1_local_var_vector()
{
    int4 vec0_ = int4(int(1), int(2), int(3), int(4));

    int x = vec0_.x;
    int y = vec0_.y;
    return;
}

float3x2 GetMatmOnBaz(Baz obj) {
    return float3x2(obj.m_0, obj.m_1, obj.m_2);
}

void SetMatmOnBaz(Baz obj, float3x2 mat) {
    obj.m_0 = mat[0];
    obj.m_1 = mat[1];
    obj.m_2 = mat[2];
}

void SetMatVecmOnBaz(Baz obj, float2 vec, uint mat_idx) {
    switch(mat_idx) {
    case 0: { obj.m_0 = vec; break; }
    case 1: { obj.m_1 = vec; break; }
    case 2: { obj.m_2 = vec; break; }
    }
}

void SetMatScalarmOnBaz(Baz obj, float scalar, uint mat_idx, uint vec_idx) {
    switch(mat_idx) {
    case 0: { obj.m_0[vec_idx] = scalar; break; }
    case 1: { obj.m_1[vec_idx] = scalar; break; }
    case 2: { obj.m_2[vec_idx] = scalar; break; }
    }
}

void pattern2_storage_buffer_matrix()
{
    float l3_ = GetMatmOnBaz(baz)[0].y;
    return;
}

void pattern3_read_modify_write()
{
    int4 vec0_1 = int4(int(1), int(2), int(3), int(4));

    int _e8 = vec0_1.y;
    vec0_1.y = asint(asuint(_e8) + asuint(int(1)));
    return;
}
