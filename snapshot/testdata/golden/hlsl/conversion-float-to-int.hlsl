static const half MIN_F16 = 3.18727e-319;
static const half MAX_F16 = 1.5683e-319;
static const float MIN_F32 = -3.4028235e+38;
static const float MAX_F32 = 3.4028235e+38;
static const double MIN_F64 = -1.7976931348623157e+308;
static const double MAX_F64 = 1.7976931348623157e+308;
static const float MIN_ABSTRACT_FLOAT = -1.#INF;
static const float MAX_ABSTRACT_FLOAT = 1.#INF;

void test_const_eval()
{
    int min_f16_to_i32 = -65504;
    int max_f16_to_i32 = 65504;
    uint min_f16_to_u32 = 0u;
    uint max_f16_to_u32 = 65504u;
    int64_t min_f16_to_i64 = -65504L;
    int64_t max_f16_to_i64 = 65504L;
    uint64_t min_f16_to_u64 = 0UL;
    uint64_t max_f16_to_u64 = 65504UL;
    int min_f32_to_i32 = -2147483648;
    int max_f32_to_i32 = 2147483520;
    uint min_f32_to_u32 = 0u;
    uint max_f32_to_u32 = 4294967040u;
    int64_t min_f32_to_i64 = -9223372036854775808L;
    int64_t max_f32_to_i64 = 9223371487098961920L;
    uint64_t min_f32_to_u64 = 0UL;
    uint64_t max_f32_to_u64 = 18446742974197923840UL;
    int64_t min_f64_to_i64 = -9223372036854775808L;
    int64_t max_f64_to_i64 = 9223372036854774784L;
    uint64_t min_f64_to_u64 = 0UL;
    uint64_t max_f64_to_u64 = 18446744073709549568UL;
    int min_abstract_float_to_i32 = -2147483648;
    int max_abstract_float_to_i32 = 2147483647;
    uint min_abstract_float_to_u32 = 0u;
    uint max_abstract_float_to_u32 = 4294967295u;
    int64_t min_abstract_float_to_i64 = -9223372036854775808L;
    int64_t max_abstract_float_to_i64 = 9223372036854774784L;
    uint64_t min_abstract_float_to_u64 = 0UL;
    uint64_t max_abstract_float_to_u64 = 18446744073709549568UL;
    
    return;
}

int test_f16_to_i32(half f)
{
    int _e1 = (int)(f);
    return _e1;
}

uint test_f16_to_u32(half f)
{
    uint _e1 = (uint)(f);
    return _e1;
}

int64_t test_f16_to_i64(half f)
{
    int64_t _e1 = (int64_t)(f);
    return _e1;
}

uint64_t test_f16_to_u64(half f)
{
    uint64_t _e1 = (uint64_t)(f);
    return _e1;
}

int test_f32_to_i32(float f)
{
    int _e1 = (int)(f);
    return _e1;
}

uint test_f32_to_u32(float f)
{
    uint _e1 = (uint)(f);
    return _e1;
}

int64_t test_f32_to_i64(float f)
{
    int64_t _e1 = (int64_t)(f);
    return _e1;
}

uint64_t test_f32_to_u64(float f)
{
    uint64_t _e1 = (uint64_t)(f);
    return _e1;
}

int test_f64_to_i32(double f)
{
    int _e1 = (int)(f);
    return _e1;
}

uint test_f64_to_u32(double f)
{
    uint _e1 = (uint)(f);
    return _e1;
}

int64_t test_f64_to_i64(double f)
{
    int64_t _e1 = (int64_t)(f);
    return _e1;
}

uint64_t test_f64_to_u64(double f)
{
    uint64_t _e1 = (uint64_t)(f);
    return _e1;
}

int2 test_f16_to_i32_vec(half2 f)
{
    int2 _e1 = (int)(f);
    return _e1;
}

uint2 test_f16_to_u32_vec(half2 f)
{
    uint2 _e1 = (uint)(f);
    return _e1;
}

int64_t2 test_f16_to_i64_vec(half2 f)
{
    int64_t2 _e1 = (int64_t)(f);
    return _e1;
}

uint64_t2 test_f16_to_u64_vec(half2 f)
{
    uint64_t2 _e1 = (uint64_t)(f);
    return _e1;
}

int2 test_f32_to_i32_vec(float2 f)
{
    int2 _e1 = (int)(f);
    return _e1;
}

uint2 test_f32_to_u32_vec(float2 f)
{
    uint2 _e1 = (uint)(f);
    return _e1;
}

int64_t2 test_f32_to_i64_vec(float2 f)
{
    int64_t2 _e1 = (int64_t)(f);
    return _e1;
}

uint64_t2 test_f32_to_u64_vec(float2 f)
{
    uint64_t2 _e1 = (uint64_t)(f);
    return _e1;
}

int2 test_f64_to_i32_vec(double2 f)
{
    int2 _e1 = (int)(f);
    return _e1;
}

uint2 test_f64_to_u32_vec(double2 f)
{
    uint2 _e1 = (uint)(f);
    return _e1;
}

int64_t2 test_f64_to_i64_vec(double2 f)
{
    int64_t2 _e1 = (int64_t)(f);
    return _e1;
}

uint64_t2 test_f64_to_u64_vec(double2 f)
{
    uint64_t2 _e1 = (uint64_t)(f);
    return _e1;
}

[numthreads(1, 1, 1)]
void main()
{
    test_const_eval();
    int _cr1 = test_f16_to_i32(1.0);
    uint _cr3 = test_f16_to_u32(1.0);
    int64_t _cr5 = test_f16_to_i64(1.0);
    uint64_t _cr7 = test_f16_to_u64(1.0);
    int _cr9 = test_f32_to_i32(1.0);
    uint _cr11 = test_f32_to_u32(1.0);
    int64_t _cr13 = test_f32_to_i64(1.0);
    uint64_t _cr15 = test_f32_to_u64(1.0);
    int _cr17 = test_f64_to_i32(1.0);
    uint _cr19 = test_f64_to_u32(1.0);
    int64_t _cr21 = test_f64_to_i64(1.0);
    uint64_t _cr23 = test_f64_to_u64(1.0);
    half2 _e26 = half2(1.0, 2.0);
    int2 _cr27 = test_f16_to_i32_vec(_e26);
    half2 _e30 = half2(1.0, 2.0);
    uint2 _cr31 = test_f16_to_u32_vec(_e30);
    half2 _e34 = half2(1.0, 2.0);
    int64_t2 _cr35 = test_f16_to_i64_vec(_e34);
    half2 _e38 = half2(1.0, 2.0);
    uint64_t2 _cr39 = test_f16_to_u64_vec(_e38);
    float2 _e42 = float2(1.0, 2.0);
    int2 _cr43 = test_f32_to_i32_vec(_e42);
    float2 _e46 = float2(1.0, 2.0);
    uint2 _cr47 = test_f32_to_u32_vec(_e46);
    float2 _e50 = float2(1.0, 2.0);
    int64_t2 _cr51 = test_f32_to_i64_vec(_e50);
    float2 _e54 = float2(1.0, 2.0);
    uint64_t2 _cr55 = test_f32_to_u64_vec(_e54);
    double2 _e58 = double2(1.0, 2.0);
    int2 _cr59 = test_f64_to_i32_vec(_e58);
    double2 _e62 = double2(1.0, 2.0);
    uint2 _cr63 = test_f64_to_u32_vec(_e62);
    double2 _e66 = double2(1.0, 2.0);
    int64_t2 _cr67 = test_f64_to_i64_vec(_e66);
    double2 _e70 = double2(1.0, 2.0);
    uint64_t2 _cr71 = test_f64_to_u64_vec(_e70);
    return;
}
