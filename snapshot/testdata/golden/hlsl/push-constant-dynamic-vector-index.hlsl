struct Immediates {
    float4 v;
    row_major float4x4 m;
};

ConstantBuffer<Immediates> im: register(b0);
RWByteAddressBuffer out_ : register(u0);

[numthreads(1, 1, 1)]
void main(uint idx : SV_GroupIndex)
{
    float v_c = im.v.x;
    float v_v = im.v[min(uint(idx), 3u)];
    float m_cc = im.m[0].x;
    float m_cv = im.m[0][min(uint(idx), 3u)];
    out_.Store(0, asuint(v_c));
    out_.Store(4, asuint(v_v));
    out_.Store(8, asuint(m_cc));
    out_.Store(12, asuint(m_cv));
    return;
}
