struct Inner {
    float value;
    uint flag;
};

struct Outer {
    float3 position;
    Inner inner;
    float scale;
    int _end_pad_0;
    int _end_pad_1;
};

float process_struct(Outer s)
{
    return ((s.position.x * s.scale) + s.inner.value);
}

Inner ConstructInner(float arg0, uint arg1) {
    Inner ret = (Inner)0;
    ret.value = arg0;
    ret.flag = arg1;
    return ret;
}

Outer ConstructOuter(float3 arg0, Inner arg1, float arg2) {
    Outer ret = (Outer)0;
    ret.position = arg0;
    ret.inner = arg1;
    ret.scale = arg2;
    return ret;
}

[numthreads(1, 1, 1)]
void main()
{
    Outer mutable_outer = (Outer)0;

    Inner inner = ConstructInner(3.14, 1u);
    Outer outer = ConstructOuter(float3(1.0, 2.0, 3.0), inner, 2.0);
    float3 pos = outer.position;
    float val = outer.inner.value;
    uint flag = outer.inner.flag;
    mutable_outer = outer;
    mutable_outer.scale = 5.0;
    mutable_outer.inner.value = 42.0;
    Outer _e20 = mutable_outer;
    const float _e21 = process_struct(_e20);
    return;
}
