ByteAddressBuffer input : register(t0);
RWByteAddressBuffer output : register(u1);

float4 ZeroValuefloat4() {
    return (float4)0;
}

[numthreads(1, 1, 1)]
void main()
{
    uint t = 0u;
    float4 acc_noinit = (float4)0;
    float4 acc_init = (float4)0;
    uint d = (uint)0;

    uint2 loop_bound = uint2(4294967295u, 4294967295u);
    bool loop_init = true;
    while(true) {
        if (all(loop_bound == uint2(0u, 0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        if (!loop_init) {
            uint _e46 = t;
            t = (_e46 + 1u);
        }
        loop_init = false;
        uint _e2 = t;
        if ((_e2 < 4u)) {
        } else {
            break;
        }
        {
            acc_init = ZeroValuefloat4();
            d = 0u;
            uint2 loop_bound_1 = uint2(4294967295u, 4294967295u);
            bool loop_init_1 = true;
            while(true) {
                if (all(loop_bound_1 == uint2(0u, 0u))) { break; }
                loop_bound_1 -= uint2(loop_bound_1.y == 0u, 1u);
                if (!loop_init_1) {
                    uint _e27 = d;
                    d = (_e27 + 1u);
                }
                loop_init_1 = false;
                uint _e10 = d;
                if ((_e10 < 16u)) {
                } else {
                    break;
                }
                {
                    uint _e14 = t;
                    uint _e17 = d;
                    float _e20 = asfloat(input.Load(((_e14 * 16u) + _e17)*4));
                    float4 v = (_e20).xxxx;
                    float4 _e22 = acc_noinit;
                    acc_noinit = (_e22 + v);
                    float4 _e24 = acc_init;
                    acc_init = (_e24 + v);
                }
            }
            uint _e30 = t;
            float _e35 = acc_noinit.x;
            output.Store((_e30 * 2u)*4, asuint(_e35));
            uint _e37 = t;
            float _e44 = acc_init.x;
            output.Store(((_e37 * 2u) + 1u)*4, asuint(_e44));
        }
    }
    return;
}
