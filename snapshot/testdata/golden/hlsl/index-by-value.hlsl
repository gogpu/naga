int index_arg_array(int a[5], int i)
{
    return a[min(uint(i), 4u)];
}

typedef int ret_Constructarray2_int_[2];
ret_Constructarray2_int_ Constructarray2_int_(int arg0, int arg1) {
    int ret[2] = { arg0, arg1 };
    return ret;
}

typedef int ret_Constructarray2_array2_int__[2][2];
ret_Constructarray2_array2_int__ Constructarray2_array2_int__(int arg0[2], int arg1[2]) {
    int ret[2][2] = { arg0, arg1 };
    return ret;
}

int index_let_array(int i_1, int j)
{
    int a_1[2][2] = Constructarray2_array2_int__(Constructarray2_int_(int(1), int(2)), Constructarray2_int_(int(3), int(4)));
    return a_1[min(uint(i_1), 1u)][min(uint(j), 1u)];
}

float index_let_matrix(int i_2, int j_1)
{
    float2x2 a_2 = float2x2(float2(1.0, 2.0), float2(3.0, 4.0));
    return a_2[min(uint(i_2), 1u)][min(uint(j_1), 1u)];
}

typedef int ret_Constructarray5_int_[5];
ret_Constructarray5_int_ Constructarray5_int_(int arg0, int arg1, int arg2, int arg3, int arg4) {
    int ret[5] = { arg0, arg1, arg2, arg3, arg4 };
    return ret;
}

float4 index_let_array_1d(uint vi_1)
{
    int arr[5] = Constructarray5_int_(int(1), int(2), int(3), int(4), int(5));
    int value = arr[min(uint(vi_1), 4u)];
    return float4((value).xxxx);
}

float4 main(uint vi : SV_VertexID) : SV_Position
{
    const int _e8 = index_arg_array(Constructarray5_int_(int(1), int(2), int(3), int(4), int(5)), int(6));
    const int _e11 = index_let_array(int(1), int(2));
    const float _e14 = index_let_matrix(int(1), int(2));
    const float4 _e15 = index_let_array_1d(vi);
    return _e15;
}
