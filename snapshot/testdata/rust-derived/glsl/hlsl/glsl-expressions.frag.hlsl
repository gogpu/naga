struct BST {
    int data;
};

struct TestStruct {
    uint4 array[2];
};

struct FragmentOutput {
    float4 o_color : SV_Target0;
};

typedef uint4 ret_Constructarray2_uint4_[2];
ret_Constructarray2_uint4_ Constructarray2_uint4_(uint4 arg0, uint4 arg1) {
    uint4 ret[2] = { arg0, arg1 };
    return ret;
}

TestStruct ConstructTestStruct(uint4 arg0[2]) {
    TestStruct ret = (TestStruct)0;
    ret.array = arg0;
    return ret;
}

static const TestStruct strct = ConstructTestStruct(Constructarray2_uint4_((0u).xxxx, (1u).xxxx));

static float global = (float)0;
RWByteAddressBuffer global_1 : register(u0);
static float4 o_color = (float4)0;

void testBinOpVecFloat(float4 a, float b)
{
    float4 a_1 = (float4)0;
    float b_1 = (float)0;
    float4 v = (float4)0;

    a_1 = a;
    b_1 = b;
    float4 _e5 = a_1;
    v = (_e5 * 2.0);
    float4 _e8 = a_1;
    v = (_e8 / (2.0).xxxx);
    float4 _e12 = a_1;
    v = (_e12 + (2.0).xxxx);
    float4 _e16 = a_1;
    v = (_e16 - (2.0).xxxx);
    return;
}

void testBinOpFloatVec(float4 a_2, float b_2)
{
    float4 a_3 = (float4)0;
    float b_3 = (float)0;
    float4 v_1 = (float4)0;

    a_3 = a_2;
    b_3 = b_2;
    float4 _e5 = a_3;
    float _e6 = b_3;
    v_1 = (_e5 * _e6);
    float4 _e8 = a_3;
    float _e9 = b_3;
    v_1 = (_e8 / (_e9).xxxx);
    float4 _e12 = a_3;
    float _e13 = b_3;
    v_1 = (_e12 + (_e13).xxxx);
    float4 _e16 = a_3;
    float _e17 = b_3;
    v_1 = (_e16 - (_e17).xxxx);
    return;
}

int4 naga_div(int4 lhs, int4 rhs) {
    return lhs / (((lhs == int(-2147483647 - 1) & rhs == -1) | (rhs == 0)) ? 1 : rhs);
}

void testBinOpIVecInt(int4 a_4, int b_4)
{
    int4 a_5 = (int4)0;
    int b_5 = (int)0;
    int4 v_2 = (int4)0;

    a_5 = a_4;
    b_5 = b_4;
    int4 _e5 = a_5;
    int _e6 = b_5;
    v_2 = asint(asuint(_e5) * asuint(_e6));
    int4 _e8 = a_5;
    int _e9 = b_5;
    v_2 = naga_div(_e8, (_e9).xxxx);
    int4 _e12 = a_5;
    int _e13 = b_5;
    v_2 = asint(asuint(_e12) + asuint((_e13).xxxx));
    int4 _e16 = a_5;
    int _e17 = b_5;
    v_2 = asint(asuint(_e16) - asuint((_e17).xxxx));
    int4 _e20 = a_5;
    int _e21 = b_5;
    v_2 = (_e20 & (_e21).xxxx);
    int4 _e24 = a_5;
    int _e25 = b_5;
    v_2 = (_e24 | (_e25).xxxx);
    int4 _e28 = a_5;
    int _e29 = b_5;
    v_2 = (_e28 ^ (_e29).xxxx);
    int4 _e32 = a_5;
    int _e33 = b_5;
    v_2 = (_e32 >> (uint(_e33)).xxxx);
    int4 _e37 = a_5;
    int _e38 = b_5;
    v_2 = (_e37 << (uint(_e38)).xxxx);
    return;
}

void testBinOpIntIVec(int a_6, int4 b_6)
{
    int a_7 = (int)0;
    int4 b_7 = (int4)0;
    int4 v_3 = (int4)0;

    a_7 = a_6;
    b_7 = b_6;
    int _e5 = a_7;
    int4 _e6 = b_7;
    v_3 = asint(asuint(_e5) * asuint(_e6));
    int _e8 = a_7;
    int4 _e9 = b_7;
    v_3 = asint(asuint((_e8).xxxx) + asuint(_e9));
    int _e12 = a_7;
    int4 _e13 = b_7;
    v_3 = asint(asuint((_e12).xxxx) - asuint(_e13));
    int _e16 = a_7;
    int4 _e17 = b_7;
    v_3 = ((_e16).xxxx & _e17);
    int _e20 = a_7;
    int4 _e21 = b_7;
    v_3 = ((_e20).xxxx | _e21);
    int _e24 = a_7;
    int4 _e25 = b_7;
    v_3 = ((_e24).xxxx ^ _e25);
    return;
}

uint4 naga_div(uint4 lhs, uint4 rhs) {
    return lhs / (rhs == 0u ? 1u : rhs);
}

void testBinOpUVecUint(uint4 a_8, uint b_8)
{
    uint4 a_9 = (uint4)0;
    uint b_9 = (uint)0;
    uint4 v_4 = (uint4)0;

    a_9 = a_8;
    b_9 = b_8;
    uint4 _e5 = a_9;
    uint _e6 = b_9;
    v_4 = (_e5 * _e6);
    uint4 _e8 = a_9;
    uint _e9 = b_9;
    v_4 = naga_div(_e8, (_e9).xxxx);
    uint4 _e12 = a_9;
    uint _e13 = b_9;
    v_4 = (_e12 + (_e13).xxxx);
    uint4 _e16 = a_9;
    uint _e17 = b_9;
    v_4 = (_e16 - (_e17).xxxx);
    uint4 _e20 = a_9;
    uint _e21 = b_9;
    v_4 = (_e20 & (_e21).xxxx);
    uint4 _e24 = a_9;
    uint _e25 = b_9;
    v_4 = (_e24 | (_e25).xxxx);
    uint4 _e28 = a_9;
    uint _e29 = b_9;
    v_4 = (_e28 ^ (_e29).xxxx);
    uint4 _e32 = a_9;
    uint _e33 = b_9;
    v_4 = (_e32 >> (_e33).xxxx);
    uint4 _e36 = a_9;
    uint _e37 = b_9;
    v_4 = (_e36 << (_e37).xxxx);
    return;
}

void testBinOpUintUVec(uint a_10, uint4 b_10)
{
    uint a_11 = (uint)0;
    uint4 b_11 = (uint4)0;
    uint4 v_5 = (uint4)0;

    a_11 = a_10;
    b_11 = b_10;
    uint _e5 = a_11;
    uint4 _e6 = b_11;
    v_5 = (_e5 * _e6);
    uint _e8 = a_11;
    uint4 _e9 = b_11;
    v_5 = ((_e8).xxxx + _e9);
    uint _e12 = a_11;
    uint4 _e13 = b_11;
    v_5 = ((_e12).xxxx - _e13);
    uint _e16 = a_11;
    uint4 _e17 = b_11;
    v_5 = ((_e16).xxxx & _e17);
    uint _e20 = a_11;
    uint4 _e21 = b_11;
    v_5 = ((_e20).xxxx | _e21);
    uint _e24 = a_11;
    uint4 _e25 = b_11;
    v_5 = ((_e24).xxxx ^ _e25);
    return;
}

void testBinOpMatMat(float3x3 a_12, float3x3 b_12)
{
    float3x3 a_13 = (float3x3)0;
    float3x3 b_13 = (float3x3)0;
    float3x3 v_6 = (float3x3)0;
    bool c = (bool)0;

    a_13 = a_12;
    b_13 = b_12;
    float3x3 _e6 = a_13;
    float3x3 _e7 = b_13;
    v_6 = float3x3((_e6[0] / _e7[0]), (_e6[1] / _e7[1]), (_e6[2] / _e7[2]));
    float3x3 _e18 = a_13;
    float3x3 _e19 = b_13;
    v_6 = mul(_e19, _e18);
    float3x3 _e21 = a_13;
    float3x3 _e22 = b_13;
    v_6 = (_e21 + _e22);
    float3x3 _e24 = a_13;
    float3x3 _e25 = b_13;
    v_6 = (_e24 - _e25);
    float3x3 _e27 = a_13;
    float3x3 _e28 = b_13;
    c = (all((_e27[2] == _e28[2])) && (all((_e27[1] == _e28[1])) && all((_e27[0] == _e28[0]))));
    float3x3 _e43 = a_13;
    float3x3 _e44 = b_13;
    c = (any((_e43[2] != _e44[2])) || (any((_e43[1] != _e44[1])) || any((_e43[0] != _e44[0]))));
    return;
}

void testBinOpMatFloat(float a_14, float3x3 b_14)
{
    float a_15 = (float)0;
    float3x3 b_15 = (float3x3)0;
    float3x3 v_7 = (float3x3)0;

    a_15 = a_14;
    b_15 = b_14;
    float _e5 = a_15;
    float3x3 _e6 = b_15;
    float3 _e7 = (_e5).xxx;
    v_7 = float3x3((_e7 / _e6[0]), (_e7 / _e6[1]), (_e7 / _e6[2]));
    float _e15 = a_15;
    float3x3 _e16 = b_15;
    v_7 = mul(_e16, _e15);
    float _e18 = a_15;
    float3x3 _e19 = b_15;
    float3 _e20 = (_e18).xxx;
    v_7 = float3x3((_e20 + _e19[0]), (_e20 + _e19[1]), (_e20 + _e19[2]));
    float _e28 = a_15;
    float3x3 _e29 = b_15;
    float3 _e30 = (_e28).xxx;
    v_7 = float3x3((_e30 - _e29[0]), (_e30 - _e29[1]), (_e30 - _e29[2]));
    float3x3 _e38 = b_15;
    float _e39 = a_15;
    float3 _e40 = (_e39).xxx;
    v_7 = float3x3((_e38[0] / _e40), (_e38[1] / _e40), (_e38[2] / _e40));
    float3x3 _e48 = b_15;
    float _e49 = a_15;
    v_7 = mul(_e49, _e48);
    float3x3 _e51 = b_15;
    float _e52 = a_15;
    float3 _e53 = (_e52).xxx;
    v_7 = float3x3((_e51[0] + _e53), (_e51[1] + _e53), (_e51[2] + _e53));
    float3x3 _e61 = b_15;
    float _e62 = a_15;
    float3 _e63 = (_e62).xxx;
    v_7 = float3x3((_e61[0] - _e63), (_e61[1] - _e63), (_e61[2] - _e63));
    return;
}

void testUnaryOpMat(float3x3 a_16)
{
    float3x3 a_17 = (float3x3)0;
    float3x3 v_8 = (float3x3)0;

    a_17 = a_16;
    float3x3 _e3 = a_17;
    v_8 = mul(_e3, -1.0);
    float3x3 _e6 = a_17;
    float3 _e8 = (1.0).xxx;
    float3x3 _e10 = (_e6 - float3x3(_e8, _e8, _e8));
    a_17 = _e10;
    v_8 = _e10;
    float3x3 _e11 = a_17;
    float3 _e13 = (1.0).xxx;
    a_17 = (_e11 - float3x3(_e13, _e13, _e13));
    v_8 = _e11;
    return;
}

BST ConstructBST(int arg0) {
    BST ret = (BST)0;
    ret.data = arg0;
    return ret;
}

void testStructConstructor()
{
    BST tree = ConstructBST(int(1));

    return;
}

void testNonScalarToScalarConstructor()
{
    float f = 1.0;

    return;
}

typedef float ret_Constructarray1_float_[1];
ret_Constructarray1_float_ Constructarray1_float_(float arg0) {
    float ret[1] = { arg0 };
    return ret;
}

void testArrayConstructor()
{
    float tree_1[1] = Constructarray1_float_(0.0);

    return;
}

void testFreestandingConstructor()
{
    return;
}

void testNonImplicitCastVectorCast()
{
    uint a_18 = 1u;
    int4 b_16 = (int4)0;

    uint _e2 = a_18;
    b_16 = (int(_e2)).xxxx;
    return;
}

void privatePointer(inout float a_19)
{
    return;
}

void ternary(bool a_20)
{
    bool a_21 = (bool)0;
    uint local = (uint)0;
    uint b_17 = (uint)0;
    uint local_1 = (uint)0;
    uint c_1 = (uint)0;
    uint local_2 = (uint)0;
    uint local_3 = (uint)0;
    uint local_4 = (uint)0;
    uint nested = (uint)0;

    a_21 = a_20;
    bool _e2 = a_21;
    if (_e2) {
        local = 0u;
    } else {
        local = 1u;
    }
    uint _e6 = local;
    b_17 = _e6;
    bool _e8 = a_21;
    if (_e8) {
        local_1 = 0u;
    } else {
        local_1 = 1u;
    }
    uint _e12 = local_1;
    c_1 = _e12;
    bool _e14 = a_21;
    if (_e14) {
        bool _e15 = a_21;
        if (_e15) {
            bool _e16 = a_21;
            if (_e16) {
                local_2 = 2u;
            } else {
                local_2 = 3u;
            }
            uint _e20 = local_2;
            local_3 = _e20;
        } else {
            local_3 = 4u;
        }
        uint _e23 = local_3;
        local_4 = _e23;
    } else {
        local_4 = 5u;
    }
    uint _e26 = local_4;
    nested = _e26;
    return;
}

void testMatrixMultiplication(float4x3 a_22, float4x4 b_18)
{
    float4x3 a_23 = (float4x3)0;
    float4x4 b_19 = (float4x4)0;
    float4x3 c_2 = (float4x3)0;

    a_23 = a_22;
    b_19 = b_18;
    float4x3 _e4 = a_23;
    float4x4 _e5 = b_19;
    c_2 = mul(_e5, _e4);
    return;
}

uint NagaBufferLengthRW(RWByteAddressBuffer buffer)
{
    uint ret;
    buffer.GetDimensions(ret);
    return ret;
}

void testLength()
{
    int len = (int)0;

    len = int(((NagaBufferLengthRW(global_1) - 0) / 4));
    return;
}

void testConstantLength(float a_24[4])
{
    float a_25[4] = (float[4])0;
    int len_1 = int(4);

    a_25 = a_24;
    return;
}

void indexConstantNonConstantIndex(int i)
{
    int i_1 = (int)0;
    TestStruct local_5 = strct;
    uint4 a_26 = (uint4)0;

    i_1 = i;
    int _e2 = i_1;
    uint4 _e7 = local_5.array[min(uint(_e2), 1u)];
    a_26 = _e7;
    return;
}

void testSwizzleWrites(float3 a_27)
{
    float3 a_28 = (float3)0;

    a_28 = a_27;
    a_28.z = 3.0;
    a_28.x = 4.0;
    float3 _e6 = a_28;
    float2 _e9 = (_e6.xy * 5.0);
    a_28.x = _e9.x;
    a_28.y = _e9.y;
    float3 _e14 = a_28;
    float2 _e18 = (_e14.zy + (1.0).xx);
    a_28.z = _e18.x;
    a_28.y = _e18.y;
    return;
}

typedef float ret_Constructarray4_float_[4];
ret_Constructarray4_float_ Constructarray4_float_(float arg0, float arg1, float arg2, float arg3) {
    float ret[4] = { arg0, arg1, arg2, arg3 };
    return ret;
}

void main_1()
{
    float local_6 = (float)0;

    testBinOpVecFloat((0.0).xxxx, 1.0);
    testBinOpFloatVec((0.0).xxxx, 1.0);
    testBinOpIVecInt((int(0)).xxxx, int(1));
    testBinOpIntIVec(int(1), (int(0)).xxxx);
    testBinOpUVecUint((0u).xxxx, 1u);
    testBinOpUintUVec(1u, (0u).xxxx);
    testBinOpMatMat(float3x3(float3(0.0, 0.0, 0.0), float3(0.0, 0.0, 0.0), float3(0.0, 0.0, 0.0)), float3x3(float3(1.0, 0.0, 0.0), float3(0.0, 1.0, 0.0), float3(0.0, 0.0, 1.0)));
    testBinOpMatFloat(1.0, float3x3(float3(1.0, 0.0, 0.0), float3(0.0, 1.0, 0.0), float3(0.0, 0.0, 1.0)));
    testUnaryOpMat(float3x3(float3(1.0, 0.0, 0.0), float3(0.0, 1.0, 0.0), float3(0.0, 0.0, 1.0)));
    testStructConstructor();
    testNonScalarToScalarConstructor();
    testArrayConstructor();
    testFreestandingConstructor();
    testNonImplicitCastVectorCast();
    float _e45 = global;
    local_6 = _e45;
    privatePointer(local_6);
    float _e47 = local_6;
    global = _e47;
    ternary(false);
    testMatrixMultiplication(float4x3(float3(0.0, 0.0, 0.0), float3(0.0, 0.0, 0.0), float3(0.0, 0.0, 0.0), float3(0.0, 0.0, 0.0)), float4x4(float4(1.0, 0.0, 0.0, 0.0), float4(0.0, 1.0, 0.0, 0.0), float4(0.0, 0.0, 1.0, 0.0), float4(0.0, 0.0, 0.0, 1.0)));
    testLength();
    testConstantLength(Constructarray4_float_(0.0, 1.0, 2.0, 3.0));
    indexConstantNonConstantIndex(int(1));
    testSwizzleWrites((0.0).xxx);
    o_color.x = 1.0;
    o_color.y = 1.0;
    o_color.z = 1.0;
    o_color.w = 1.0;
    return;
}

FragmentOutput ConstructFragmentOutput(float4 arg0) {
    FragmentOutput ret = (FragmentOutput)0;
    ret.o_color = arg0;
    return ret;
}

FragmentOutput main()
{
    main_1();
    float4 _e1 = o_color;
    const FragmentOutput fragmentoutput = ConstructFragmentOutput(_e1);
    return fragmentoutput;
}
