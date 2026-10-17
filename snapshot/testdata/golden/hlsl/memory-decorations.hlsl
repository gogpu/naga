RWByteAddressBuffer coherent_buf : register(u0);
RWByteAddressBuffer volatile_buf : register(u1);
RWByteAddressBuffer both_buf : register(u2);
RWByteAddressBuffer plain_buf : register(u3);

[numthreads(1, 1, 1)]
void main()
{
    uint _e6 = asuint(volatile_buf.Load(0+0));
    coherent_buf.Store(0+0, asuint(_e6));
    uint _e13 = asuint(plain_buf.Load(0+0));
    both_buf.Store(0+0, asuint(_e13));
    return;
}
