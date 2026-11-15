RWByteAddressBuffer testBuffer : register(u0);
ByteAddressBuffer testBufferReadOnly : register(t2);

void main_1()
{
    uint a = (uint)0;
    uint b = (uint)0;

    uint _e4 = asuint(testBuffer.Load(0+0));
    a = _e4;
    testBuffer.Store(int(1)*4+0, asuint(2u));
    uint _e12 = asuint(testBufferReadOnly.Load(0+0));
    b = _e12;
    return;
}

void main()
{
    main_1();
    return;
}
