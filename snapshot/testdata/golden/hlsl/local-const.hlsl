static const int gb = int(4);
static const uint gc = 4u;
static const float gd = 4.0;

void const_in_fn()
{
    return;
}

[numthreads(1, 1, 1)]
void main()
{
    const_in_fn();
    return;
}
