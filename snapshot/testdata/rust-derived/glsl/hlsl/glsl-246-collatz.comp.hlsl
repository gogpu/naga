RWByteAddressBuffer global : register(u0);
static uint3 gl_GlobalInvocationID_1 = (uint3)0;

uint naga_div(uint lhs, uint rhs) {
    return lhs / (rhs == 0u ? 1u : rhs);
}

uint collatz_iterations(uint n)
{
    uint n_1 = (uint)0;
    uint i = 0u;

    n_1 = n;
    uint2 loop_bound = uint2(4294967295u, 4294967295u);
    while(true) {
        if (all(loop_bound == uint2(0u, 0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        uint _e4 = n_1;
        if (!((_e4 != 1u))) {
            break;
        }
        {
            uint _e8 = n_1;
            float _e9 = float(_e8);
            if (((_e9 - (floor((_e9 / 2.0)) * 2.0)) == 0.0)) {
                {
                    uint _e17 = n_1;
                    n_1 = naga_div(_e17, 2u);
                }
            } else {
                {
                    uint _e20 = n_1;
                    n_1 = ((3u * _e20) + 1u);
                }
            }
            uint _e25 = i;
            i = (_e25 + 1u);
        }
    }
    uint _e28 = i;
    return _e28;
}

void main_1()
{
    uint index = (uint)0;

    uint3 _e3 = gl_GlobalInvocationID_1;
    index = _e3.x;
    uint _e6 = index;
    uint _e8 = index;
    uint _e10 = asuint(global.Load(_e8*4+0));
    const uint _e11 = collatz_iterations(_e10);
    global.Store(_e6*4+0, asuint(_e11));
    return;
}

[numthreads(1, 1, 1)]
void main(uint3 gl_GlobalInvocationID : SV_DispatchThreadID)
{
    gl_GlobalInvocationID_1 = gl_GlobalInvocationID;
    main_1();
    return;
}
