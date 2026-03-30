RWByteAddressBuffer out_ : register(u0);

typedef int ret_Constructarray2_int_[2];
ret_Constructarray2_int_ Constructarray2_int_(int arg0, int arg1) {
    int ret[2] = { arg0, arg1 };
    return ret;
}

[numthreads(1, 1, 1)]
void main()
{
    int tmp[2] = Constructarray2_int_(int(1), int(2));
    int i = int(0);

    uint2 loop_bound = uint2(4294967295u, 4294967295u);
    bool loop_init = true;
    while(true) {
        if (all(loop_bound == uint2(0u, 0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
        if (!loop_init) {
            int _e16 = i;
            i = asint(asuint(_e16) + asuint(int(1)));
        }
        loop_init = false;
        int _e6 = i;
        if ((_e6 < int(2))) {
        } else {
            break;
        }
        {
            int _e10 = i;
            int _e12 = i;
            int _e14 = tmp[min(uint(_e12), 1u)];
            out_.Store(_e10*4, asuint(_e14));
        }
    }
    return;
}
