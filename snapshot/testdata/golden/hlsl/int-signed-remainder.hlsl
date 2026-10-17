int naga_mod(int lhs, int rhs) {
    int divisor = ((lhs == int(-2147483647 - 1) & rhs == -1) | (rhs == 0)) ? 1 : rhs;
    return lhs - (lhs / divisor) * divisor;
}

int2 naga_mod(int2 lhs, int2 rhs) {
    int2 divisor = ((lhs == int(-2147483647 - 1) & rhs == -1) | (rhs == 0)) ? 1 : rhs;
    return lhs - (lhs / divisor) * divisor;
}

uint naga_mod(uint lhs, uint rhs) {
    return lhs % (rhs == 0u ? 1u : rhs);
}

[numthreads(1, 1, 1)]
void main()
{
    int mod_s = naga_mod(int(5), int(2));
    int2 mod_s_vec = naga_mod((int(5)).xx, (int(2)).xx);
    uint mod_u = naga_mod(5u, 2u);
    return;
}
