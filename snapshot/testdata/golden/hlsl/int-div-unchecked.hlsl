int naga_div(int lhs, int rhs) {
    return lhs / (((lhs == int(-2147483647 - 1) & rhs == -1) | (rhs == 0)) ? 1 : rhs);
}

int naga_mod(int lhs, int rhs) {
    int divisor = ((lhs == int(-2147483647 - 1) & rhs == -1) | (rhs == 0)) ? 1 : rhs;
    return lhs - (lhs / divisor) * divisor;
}

uint naga_div(uint lhs, uint rhs) {
    return lhs / (rhs == 0u ? 1u : rhs);
}

uint naga_mod(uint lhs, uint rhs) {
    return lhs % (rhs == 0u ? 1u : rhs);
}

[numthreads(1, 1, 1)]
void main()
{
    int div_s = naga_div(int(5), int(2));
    int mod_s = naga_mod(int(5), int(2));
    uint div_u = naga_div(5u, 2u);
    uint mod_u = naga_mod(5u, 2u);
    return;
}
